package g

import (
	"errors"
	"time"

	"github.com/cihub/seelog"
	"golang.org/x/crypto/bcrypt"
)

// User 系统登录用户
type User struct {
	Id        int    `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"` // admin | viewer
	CreatedAt string `json:"created_at"`
	LastLogin string `json:"last_login"`
}

const (
	RoleAdmin  = "admin"
	RoleViewer = "viewer"
)

// UserFull 含密码哈希的完整用户行(仅用于节点间加密同步, 不暴露给页面)
type UserFull struct {
	Username  string `json:"username"`
	Password  string `json:"password"` // bcrypt 哈希
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
	LastLogin string `json:"last_login"`
}

// OnUsersReplaced 用户表被集群同步整体替换后的回调(http 层注册, 用于踢掉旧会话)
var OnUsersReplaced func()

// touchUserRev 用户数据版本号 +1(任何用户增删改时调用, 调用方需持有 DLock)。
// Agent 据此跟随主节点同步账户密码: 版本高者胜出。
func touchUserRev() {
	now := time.Now().Format("2006-01-02 15:04:05")
	Db.Exec(`INSERT INTO users_meta(id, rev, mtime) VALUES (1, 2, ?)
		ON CONFLICT(id) DO UPDATE SET rev = rev + 1, mtime = ?`, now, now)
}

// UserRev 当前用户数据版本号(无记录时为 1, 即初始默认账号状态)
func UserRev() int64 {
	var rev int64 = 1
	DLock.Lock()
	Db.QueryRow("select rev from users_meta where id = 1").Scan(&rev)
	DLock.Unlock()
	if rev < 1 {
		rev = 1
	}
	return rev
}

// ListUsersFull 导出完整用户表(含哈希), 用于节点间同步
func ListUsersFull() ([]UserFull, error) {
	users := []UserFull{}
	DLock.Lock()
	rows, err := Db.Query("select username,password,role,ifnull(created_at,''),ifnull(last_login,'') from users order by id")
	DLock.Unlock()
	if err != nil {
		return users, err
	}
	defer rows.Close()
	for rows.Next() {
		u := UserFull{}
		if err := rows.Scan(&u.Username, &u.Password, &u.Role, &u.CreatedAt, &u.LastLogin); err != nil {
			continue
		}
		users = append(users, u)
	}
	return users, nil
}

// ReplaceUsers 用主节点同步来的用户表整体替换本地(事务内), 并写入对方版本号。
// 空列表拒绝执行, 防止把自己锁在门外。
func ReplaceUsers(users []UserFull, rev int64) error {
	if len(users) == 0 {
		return errors.New("refuse to replace with empty user list")
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	DLock.Lock()
	tx, err := Db.Begin()
	if err != nil {
		DLock.Unlock()
		return err
	}
	tx.Exec("delete from users")
	for _, u := range users {
		if u.Username == "" || u.Password == "" {
			continue
		}
		tx.Exec("insert into users(username,password,role,created_at,last_login) values(?,?,?,?,?)",
			u.Username, u.Password, u.Role, u.CreatedAt, u.LastLogin)
	}
	tx.Exec(`INSERT INTO users_meta(id, rev, mtime) VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET rev = ?, mtime = ?`, rev, now, rev, now)
	err = tx.Commit()
	DLock.Unlock()
	if err != nil {
		return err
	}
	seelog.Info("[func:ReplaceUsers] user table replaced by cluster sync (", len(users), " users, rev ", rev, ")")
	if OnUsersReplaced != nil {
		OnUsersReplaced()
	}
	return nil
}

// InitUserTable 初始化用户表, 并在首次启动时创建默认管理员 admin/admin123
func InitUserTable() {
	createSql := `CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username VARCHAR(64) NOT NULL UNIQUE,
		password VARCHAR(128) NOT NULL,
		role VARCHAR(16) NOT NULL DEFAULT 'viewer',
		created_at VARCHAR(32),
		last_login VARCHAR(32)
	)`
	DLock.Lock()
	defer DLock.Unlock()
	if _, err := Db.Exec(createSql); err != nil {
		seelog.Error("[func:InitUserTable] create table error ", err)
		return
	}
	// 用户数据版本表: 集群间账户同步的 LWW 依据(初始 rev=1)
	Db.Exec(`CREATE TABLE IF NOT EXISTS users_meta (
		id    INTEGER PRIMARY KEY CHECK (id = 1),
		rev   INTEGER NOT NULL DEFAULT 1,
		mtime VARCHAR(32)
	)`)
	Db.Exec("INSERT OR IGNORE INTO users_meta(id, rev, mtime) VALUES (1, 1, ?)",
		time.Now().Format("2006-01-02 15:04:05"))
	var cnt int
	if err := Db.QueryRow("select count(1) from users").Scan(&cnt); err != nil {
		seelog.Error("[func:InitUserTable] count error ", err)
		return
	}
	if cnt == 0 {
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		_, err := Db.Exec("insert into users(username,password,role,created_at) values(?,?,?,?)",
			"admin", string(hash), RoleAdmin, time.Now().Format("2006-01-02 15:04:05"))
		if err != nil {
			seelog.Error("[func:InitUserTable] create default admin error ", err)
			return
		}
		seelog.Info("[func:InitUserTable] default admin created: admin / admin123")
	}
}

// DefaultCredsActive 默认账号 admin/admin123 是否仍然有效(用于登录页提示)
func DefaultCredsActive() bool {
	var hash string
	DLock.Lock()
	err := Db.QueryRow("select password from users where username = 'admin'").Scan(&hash)
	DLock.Unlock()
	if err != nil {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte("admin123")) == nil
}

// VerifyUser 校验用户名密码, 成功返回用户信息
func VerifyUser(username, password string) (User, error) {
	u := User{}
	var hash string
	DLock.Lock()
	err := Db.QueryRow("select id,username,password,role,ifnull(created_at,''),ifnull(last_login,'') from users where username = ?", username).
		Scan(&u.Id, &u.Username, &hash, &u.Role, &u.CreatedAt, &u.LastLogin)
	DLock.Unlock()
	if err != nil {
		return u, errors.New("用户不存在")
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return u, errors.New("密码错误")
	}
	DLock.Lock()
	Db.Exec("update users set last_login = ? where id = ?", time.Now().Format("2006-01-02 15:04:05"), u.Id)
	DLock.Unlock()
	return u, nil
}

// GetUser 根据用户名获取用户
func GetUser(username string) (User, error) {
	u := User{}
	DLock.Lock()
	err := Db.QueryRow("select id,username,role,ifnull(created_at,''),ifnull(last_login,'') from users where username = ?", username).
		Scan(&u.Id, &u.Username, &u.Role, &u.CreatedAt, &u.LastLogin)
	DLock.Unlock()
	return u, err
}

// ListUsers 用户列表
func ListUsers() ([]User, error) {
	users := []User{}
	DLock.Lock()
	rows, err := Db.Query("select id,username,role,ifnull(created_at,''),ifnull(last_login,'') from users order by id")
	DLock.Unlock()
	if err != nil {
		return users, err
	}
	defer rows.Close()
	for rows.Next() {
		u := User{}
		if err := rows.Scan(&u.Id, &u.Username, &u.Role, &u.CreatedAt, &u.LastLogin); err != nil {
			continue
		}
		users = append(users, u)
	}
	return users, nil
}

// CreateUser 创建用户
func CreateUser(username, password, role string) error {
	if role != RoleAdmin && role != RoleViewer {
		return errors.New("非法角色")
	}
	if len(username) < 2 {
		return errors.New("用户名至少2个字符")
	}
	if len(password) < 6 {
		return errors.New("密码至少6个字符")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	DLock.Lock()
	_, err = Db.Exec("insert into users(username,password,role,created_at) values(?,?,?,?)",
		username, string(hash), role, time.Now().Format("2006-01-02 15:04:05"))
	if err == nil {
		touchUserRev()
	}
	DLock.Unlock()
	if err != nil {
		return errors.New("用户已存在或写入失败")
	}
	return nil
}

// UpdateUserRole 更新用户角色
func UpdateUserRole(username, role string) error {
	if role != RoleAdmin && role != RoleViewer {
		return errors.New("非法角色")
	}
	DLock.Lock()
	_, err := Db.Exec("update users set role = ? where username = ?", role, username)
	if err == nil {
		touchUserRev()
	}
	DLock.Unlock()
	return err
}

// UpdateUserPassword 更新用户密码
func UpdateUserPassword(username, password string) error {
	if len(password) < 6 {
		return errors.New("密码至少6个字符")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	DLock.Lock()
	_, err = Db.Exec("update users set password = ? where username = ?", string(hash), username)
	if err == nil {
		touchUserRev()
	}
	DLock.Unlock()
	return err
}

// DeleteUser 删除用户(保护最后一个管理员)
func DeleteUser(username string) error {
	u, err := GetUser(username)
	if err != nil {
		return errors.New("用户不存在")
	}
	if u.Role == RoleAdmin {
		var cnt int
		DLock.Lock()
		Db.QueryRow("select count(1) from users where role = 'admin'").Scan(&cnt)
		DLock.Unlock()
		if cnt <= 1 {
			return errors.New("不能删除最后一个管理员")
		}
	}
	DLock.Lock()
	_, err = Db.Exec("delete from users where username = ?", username)
	if err == nil {
		touchUserRev()
	}
	DLock.Unlock()
	return err
}

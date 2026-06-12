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
	DLock.Unlock()
	return err
}

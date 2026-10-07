// 构建前清理上次生成的页面与 bundle(只删生成物, 保留 html/assets 下的静态资源)
import { readdirSync, rmSync } from 'node:fs';
import { join } from 'node:path';

const out = new URL('../../html/', import.meta.url).pathname;
const pages = readdirSync(new URL('../pages/', import.meta.url).pathname).filter((f) => f.endsWith('.html'));
pages.forEach((f) => rmSync(join(out, f), { force: true }));
rmSync(join(out, 'assets/app'), { recursive: true, force: true });

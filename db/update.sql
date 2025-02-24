-- ALTER TABLE devices ADD COLUMN callsign TEXT DEFAULT '';
-- ALTER TABLE public_groups ADD COLUMN allow_callsign_ssid TEXT DEFAULT '';
-- CREATE UNIQUE INDEX idx_ssid_callsign ON devices (ssid, callsign);
-- CREATE UNIQUE INDEX idx_name_unique ON public_groups(name);
-- CREATE UNIQUE INDEX idx_phone_unique ON users(phone);
-- CREATE UNIQUE INDEX idx_callsign_unique ON users(callsign);

CREATE TABLE registers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,  -- 自增主键
    callsign TEXT NOT NULL UNIQUE,         -- 用户呼号，唯一
    name TEXT NOT NULL,                    -- 用户姓名
    phone TEXT NOT NULL,                   -- 用户手机号
    address TEXT NOT NULL,
    mail TEXT NOT NULL, 
    birthday TEXT NOT NULL,
    sex INTEGER NOT NULL,
    password TEXT NOT NULL,                -- 用户密码（加密存储）
    op_cert_path TEXT,                     -- 操作证文件路径
    license_path TEXT,                     -- 电台执照文件路径
    status INTEGER DEFAULT 1,         -- 注册状态（默认未审核）
    create_time DATETIME DEFAULT CURRENT_TIMESTAMP, -- 创建时间
    update_time DATETIME DEFAULT CURRENT_TIMESTAMP, -- 创建时间
    note TEXT NOT NULL
);


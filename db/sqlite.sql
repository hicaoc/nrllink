CREATE TABLE "devices" (
	"id"	INTEGER UNIQUE COLLATE BINARY,
	"name"	TEXT,
	"cpuid"	TEXT,
	"password"	TEXT,
	"gird"	TEXT,
	"ssid"	TEXT,
	"dev_type"	INTEGER,
	"dev_model"	INTEGER,
	"group_id"	INTEGER,
	"status"	INTEGER,
	"is_certed"	BLOB,
	"chan_name"	TEXT,
	"online_time"	TEXT,
	"create_time"	TEXT,
	"update_time"	TEXT,
	"note"	TEXT,
	PRIMARY KEY("id" AUTOINCREMENT)
);

CREATE TABLE "servers" (
	"id"	INTEGER UNIQUE,
	"name"	TEXT,
	"join_key"	TEXT,
	"cpu_type"	TEXT,
	"mem_size"	TEXT,
	"input_rate"	INTEGER,
	"output_rate"	INTEGER,
	"netcard"	TEXT,
	"ip_type"	INTEGER,
	"ip_addr"	TEXT,
	"dns_name"	TEXT,
	"server_type"	INTEGER,
	"group_list"	INTEGER,
	"ower_id"	TEXT,
	"ower_callsign"	TEXT,
	"is_online"	NUMERIC,
	"status"	INTEGER,
	"create_time"	TEXT,
	"update_time"	TEXT,
	"note"	TEXT,
	"udp_port"	INTEGER,
	PRIMARY KEY("id" AUTOINCREMENT)
);
CREATE TABLE "public_groups" (
	"id"	INTEGER UNIQUE,
	"name"	TEXT,
	"type"	INTEGER,
	"callsign"	TEXT,
	"password"	TEXT,
	"allow_cpuid"	TEXT,
	"ower_id"	INTEGER,
	"devlist"	TEXT,
	"master_server"	INTEGER,
	"slave_server"	INTEGER,
	"status"	INTEGER,
	"create_time"	TEXT,
	"update_time"	TEXT,
	"note"	TEXT,
	PRIMARY KEY("id" AUTOINCREMENT)
);

CREATE TABLE "users" (
	"id"	INTEGER UNIQUE,
	"name"	TEXT,
	"callsign"	TEXT,
	"gird"	TEXT,
	"phone"	TEXT,
	"password"	TEXT,
	"birthday"	TEXT,
	"sex"	BLOB,
	"avatar"	TEXT,
	"address"	TEXT,
	"roles"	TEXT,
	"introduction"	TEXT,
	"alarm_msg"	BLOB,
	"status"	INTEGER,
	"update_time"	TEXT,
	"last_login_time"	TEXT,
	"login_err_times"	INTEGER,
	"create_time"	TEXT,
	"openid"	TEXT,
	"nickname"	TEXT,
	"pid"	TEXT,
	"last_login_ip"	TEXT,
	PRIMARY KEY("id" AUTOINCREMENT)
);

CREATE TABLE "roles" (
	"id"	INTEGER UNIQUE,
	"name_key"	TEXT,
	"name"	TEXT,
	"description"	TEXT,
	"routess"	TEXT,
	PRIMARY KEY("id" AUTOINCREMENT)
);

CREATE TABLE "operator_log" (
	"id"	INTEGER UNIQUE,
	"timestamp"	TEXT,
	"content"	TEXT,
	"event_type"	TEXT,
	"operator"	TEXT,
	"operator_id"	INTEGER,
	PRIMARY KEY("id" AUTOINCREMENT)
);

CREATE TABLE "relay" (
	"id"	INTEGER UNIQUE,
	"name"	TEXT,
	"up_freq"	TEXT,
	"down_freq"	TEXT,
	"send_ctss"	TEXT,
	"recive_ctss"	TEXT,
	"ower_callsign"	TEXT,
	"create_time"	TEXT,
	"update_time"	TEXT,
	"status"	INTEGER,
	"note"	TEXT,
	PRIMARY KEY("id" AUTOINCREMENT)
);
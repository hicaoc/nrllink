-- Table: area
DROP TABLE devices;

CREATE TABLE devices
(
    id integer NOT NULL GENERATED BY DEFAULT AS IDENTITY ( INCREMENT 1 START 1 MINVALUE 1 MAXVALUE 2147483647 CACHE 1 ),
    name character varying COLLATE pg_catalog."default",
    cpuid character varying COLLATE pg_catalog."default",
    gird character varying COLLATE pg_catalog."default",
    callsign character varying COLLATE pg_catalog."default",
    ssid  integer DEFAULT 0 ,
    dev_type  integer DEFAULT 0 ,
    dev_model  integer DEFAULT 0 ,
    ower_id   integer DEFAULT 0 ,
   	group_id   integer DEFAULT 0 ,
    public_group_id   integer DEFAULT 0 ,
	status       integer DEFAULT 0 ,
	is_certed   boolean DEFAULT false,
	is_online     boolean DEFAULT false ,
	online_time timestamp without time zone,   
    create_time timestamp without time zone,
    update_time timestamp without time zone,
    note character varying COLLATE pg_catalog."default",
    CONSTRAINT device_pkey PRIMARY KEY (id),
    CONSTRAINT cpuid UNIQUE (cpuid)
   
)

TABLESPACE pg_default;

ALTER TABLE devices
    OWNER to postgres;


--pg 9.4

drop table   area ;

CREATE TABLE area
(
    id serial ,
    name character varying COLLATE pg_catalog."default",    
    schname character varying COLLATE pg_catalog."default",
    create_time timestamp without time zone,
    update_time timestamp without time zone,
    note character varying COLLATE pg_catalog."default",
    status integer,
 
    CONSTRAINT area_pkey PRIMARY KEY (schname)

    
)

TABLESPACE pg_default;
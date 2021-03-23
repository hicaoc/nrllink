-- Table: roles

-- DROP TABLE roles;

CREATE TABLE roles
(   
    id integer NOT NULL GENERATED BY DEFAULT AS IDENTITY ( INCREMENT 1 START 1 MINVALUE 1 MAXVALUE 2147483647 CACHE 1 ),
    name_key character varying COLLATE pg_catalog."default" NOT NULL,
    name character varying COLLATE pg_catalog."default" NOT NULL,
    description character varying COLLATE pg_catalog."default",
    routes json,
 
    CONSTRAINT roles_pkey PRIMARY KEY (name_key)
)

TABLESPACE pg_default;

ALTER TABLE roles
    OWNER to postgres;





    CREATE TABLE roles
(

    id serial   ,
    name_key character varying COLLATE pg_catalog."default" NOT NULL,
    name character varying COLLATE pg_catalog."default" NOT NULL,
    description character varying COLLATE pg_catalog."default",
    routes json,
 
    CONSTRAINT roles_pkey PRIMARY KEY (name_key)
)

TABLESPACE pg_default;

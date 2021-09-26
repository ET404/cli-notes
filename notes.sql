-- public.notes definition

-- Drop table

-- DROP TABLE public.notes;

CREATE TABLE public.notes (
    id serial NOT NULL,
    note varchar NULL,
    pubtime timestamp(0) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT id_pk PRIMARY KEY (id)
);

--dropping multiple objects
DROP SEQUENCE seq1_tbl,seq2_tbl,seq3_tbl;

ALTER TABLE public.example ALTER COLUMN id2 ADD GENERATED ALWAYS AS IDENTITY (
     SEQUENCE NAME public.example_id2_seq
     START WITH 1
     INCREMENT BY 1
     NO MINVALUE
     NO MAXVALUE
     CACHE 1
);
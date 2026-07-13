-- Documents unique per (folder, name); unnamed drafts have name = NULL.
-- Default NULLS DISTINCT semantics: any index entry with a NULL component
-- can never conflict, so many drafts may share a folder.
DROP TABLE IF EXISTS docs CASCADE;
CREATE TABLE docs (
    id        int PRIMARY KEY,
    folder_id int,
    name      text,
    c0 text, c1 text,
    CONSTRAINT docs_folder_name_uk UNIQUE (folder_id, name)
);
ALTER TABLE docs REPLICA IDENTITY FULL;

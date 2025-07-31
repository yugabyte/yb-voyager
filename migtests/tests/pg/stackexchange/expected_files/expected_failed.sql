/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX badges_date_idx ON public.badges USING btree (date ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX badges_name_idx ON public.badges USING btree (name ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX badges_user_id_idx ON public.badges USING btree (userid ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX cmnts_creation_date_idx ON public.comments USING btree (creationdate ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX cmnts_postid_idx ON public.comments USING hash (postid) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX cmnts_score_idx ON public.comments USING btree (score ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX cmnts_userid_idx ON public.comments USING btree (userid ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX ph_creation_date_idx ON public.posthistory USING btree (creationdate ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX ph_post_type_id_idx ON public.posthistory USING btree (posthistorytypeid ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX ph_postid_idx ON public.posthistory USING hash (postid) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX ph_revguid_idx ON public.posthistory USING btree (revisionguid ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX ph_userid_idx ON public.posthistory USING btree (userid ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX postlinks_post_id_idx ON public.postlinks USING btree (postid ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX postlinks_related_post_id_idx ON public.postlinks USING btree (relatedpostid ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX posts_accepted_answer_id_idx ON public.posts USING btree (acceptedanswerid ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX posts_answer_count_idx ON public.posts USING btree (answercount ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX posts_comment_count_idx ON public.posts USING btree (commentcount ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX posts_creation_date_idx ON public.posts USING btree (creationdate ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX posts_favorite_count_idx ON public.posts USING btree (favoritecount ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX posts_id_accepted_answers_id_idx ON public.posts USING btree (id ASC, acceptedanswerid) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX posts_id_parent_id_idx ON public.posts USING btree (id ASC, parentid) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX posts_id_post_type_id_idx ON public.posts USING btree (id ASC, posttypeid) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX posts_owner_user_id_creation_date_idx ON public.posts USING btree (owneruserid ASC, creationdate) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX posts_owner_user_id_idx ON public.posts USING hash (owneruserid) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX posts_parent_id_idx ON public.posts USING btree (parentid ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX posts_post_type_id_idx ON public.posts USING btree (posttypeid ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX posts_score_idx ON public.posts USING btree (score ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX posts_viewcount_idx ON public.posts USING btree (viewcount ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX posttags_postid_idx ON public.posttags USING hash (postid) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX posttags_tagid_idx ON public.posttags USING btree (tagid ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX tags_count_idx ON public.tags USING btree (count ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX tags_name_idx ON public.tags USING hash (tagname) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX user_acc_id_idx ON public.users USING hash (accountid) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX user_created_at_idx ON public.users USING btree (creationdate ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX user_display_idx ON public.users USING hash (displayname) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX user_down_votes_idx ON public.users USING btree (downvotes ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX user_up_votes_idx ON public.users USING btree (upvotes ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX usertagqa_all_qa_posts_idx ON public.usertagqa USING btree ((questions + answers) ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX usertagqa_answers_idx ON public.usertagqa USING btree (answers ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX usertagqa_questions_answers_idx ON public.usertagqa USING btree (questions ASC, answers) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX votes_creation_date_idx ON public.votes USING btree (creationdate ASC) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX votes_post_id_idx ON public.votes USING hash (postid) WITH (fillfactor='100');

/*
ERROR: unrecognized parameter "fillfactor" (SQLSTATE 22023)
File :/home/centos/yb-voyager/migtests/tests/pg/stackexchange/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX votes_type_idx ON public.votes USING btree (votetypeid ASC) WITH (fillfactor='100');


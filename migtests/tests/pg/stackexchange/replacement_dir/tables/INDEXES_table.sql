-- setting variables for current session
SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;


CREATE INDEX badges_date_idx ON public.badges USING btree (date);


CREATE INDEX badges_name_idx ON public.badges USING btree (name);


CREATE INDEX badges_user_id_idx ON public.badges USING btree (userid);


CREATE INDEX cmnts_creation_date_idx ON public.comments USING btree (creationdate);


CREATE INDEX cmnts_postid_idx ON public.comments USING hash (postid);


CREATE INDEX cmnts_score_idx ON public.comments USING btree (score);


CREATE INDEX cmnts_userid_idx ON public.comments USING btree (userid);


CREATE INDEX ph_creation_date_idx ON public.posthistory USING btree (creationdate);


CREATE INDEX ph_post_type_id_idx ON public.posthistory USING btree (posthistorytypeid);


CREATE INDEX ph_postid_idx ON public.posthistory USING hash (postid);


CREATE INDEX ph_revguid_idx ON public.posthistory USING btree (revisionguid);


CREATE INDEX ph_userid_idx ON public.posthistory USING btree (userid);


CREATE INDEX postlinks_post_id_idx ON public.postlinks USING btree (postid);


CREATE INDEX postlinks_related_post_id_idx ON public.postlinks USING btree (relatedpostid);


CREATE INDEX posts_accepted_answer_id_idx ON public.posts USING btree (acceptedanswerid);


CREATE INDEX posts_answer_count_idx ON public.posts USING btree (answercount);


CREATE INDEX posts_comment_count_idx ON public.posts USING btree (commentcount);


CREATE INDEX posts_creation_date_idx ON public.posts USING btree (creationdate);


CREATE INDEX posts_favorite_count_idx ON public.posts USING btree (favoritecount);


CREATE INDEX posts_id_accepted_answers_id_idx ON public.posts USING btree (id, acceptedanswerid);


CREATE INDEX posts_id_parent_id_idx ON public.posts USING btree (id, parentid);


CREATE INDEX posts_id_post_type_id_idx ON public.posts USING btree (id, posttypeid);


CREATE INDEX posts_owner_user_id_creation_date_idx ON public.posts USING btree (owneruserid, creationdate);


CREATE INDEX posts_owner_user_id_idx ON public.posts USING hash (owneruserid);


CREATE INDEX posts_parent_id_idx ON public.posts USING btree (parentid);


CREATE INDEX posts_post_type_id_idx ON public.posts USING btree (posttypeid);


CREATE INDEX posts_score_idx ON public.posts USING btree (score);


CREATE INDEX posts_viewcount_idx ON public.posts USING btree (viewcount);


CREATE INDEX posttags_postid_idx ON public.posttags USING hash (postid);


CREATE INDEX posttags_tagid_idx ON public.posttags USING btree (tagid);


CREATE INDEX tags_count_idx ON public.tags USING btree (count);


CREATE INDEX tags_name_idx ON public.tags USING hash (tagname);


CREATE INDEX user_acc_id_idx ON public.users USING hash (accountid);


CREATE INDEX user_created_at_idx ON public.users USING btree (creationdate);


CREATE INDEX user_display_idx ON public.users USING hash (displayname);


CREATE INDEX user_down_votes_idx ON public.users USING btree (downvotes);


CREATE INDEX user_up_votes_idx ON public.users USING btree (upvotes);


CREATE INDEX usertagqa_all_qa_posts_idx ON public.usertagqa USING btree (((questions + answers)));


CREATE INDEX usertagqa_answers_idx ON public.usertagqa USING btree (answers);


CREATE INDEX usertagqa_questions_answers_idx ON public.usertagqa USING btree (questions, answers);


CREATE INDEX votes_creation_date_idx ON public.votes USING btree (creationdate);


CREATE INDEX votes_post_id_idx ON public.votes USING hash (postid);


CREATE INDEX votes_type_idx ON public.votes USING btree (votetypeid);



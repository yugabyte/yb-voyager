SET statement_timeout TO 0;

SET lock_timeout TO 0;

SET idle_in_transaction_session_timeout TO 0;

SET transaction_timeout TO 0;

SET client_encoding TO "UTF8";

SET standard_conforming_strings TO ON;

SELECT pg_catalog.set_config('search_path', '', false);

SET xmloption TO content;

SET client_min_messages TO warning;

SET row_security TO OFF;

CREATE INDEX badges_date_idx ON public.badges USING btree (date ASC);

CREATE INDEX badges_name_idx ON public.badges USING btree (name ASC);

CREATE INDEX badges_user_id_idx ON public.badges USING btree (userid ASC);

CREATE INDEX cmnts_creation_date_idx ON public.comments USING btree (creationdate ASC);

CREATE INDEX cmnts_postid_idx ON public.comments USING hash (postid);

CREATE INDEX cmnts_score_idx ON public.comments USING btree (score ASC);

CREATE INDEX cmnts_userid_idx ON public.comments USING btree (userid ASC);

CREATE INDEX ph_creation_date_idx ON public.posthistory USING btree (creationdate ASC);

CREATE INDEX ph_post_type_id_idx ON public.posthistory USING btree (posthistorytypeid ASC);

CREATE INDEX ph_postid_idx ON public.posthistory USING hash (postid);

CREATE INDEX ph_revguid_idx ON public.posthistory USING btree (revisionguid ASC);

CREATE INDEX ph_userid_idx ON public.posthistory USING btree (userid ASC);

CREATE INDEX postlinks_post_id_idx ON public.postlinks USING btree (postid ASC);

CREATE INDEX postlinks_related_post_id_idx ON public.postlinks USING btree (relatedpostid ASC);

CREATE INDEX posts_accepted_answer_id_idx ON public.posts USING btree (acceptedanswerid ASC);

CREATE INDEX posts_answer_count_idx ON public.posts USING btree (answercount ASC);

CREATE INDEX posts_comment_count_idx ON public.posts USING btree (commentcount ASC);

CREATE INDEX posts_creation_date_idx ON public.posts USING btree (creationdate ASC);

CREATE INDEX posts_favorite_count_idx ON public.posts USING btree (favoritecount ASC);

CREATE INDEX posts_id_accepted_answers_id_idx ON public.posts USING btree (id ASC, acceptedanswerid);

CREATE INDEX posts_id_parent_id_idx ON public.posts USING btree (id ASC, parentid);

CREATE INDEX posts_id_post_type_id_idx ON public.posts USING btree (id ASC, posttypeid);

CREATE INDEX posts_owner_user_id_creation_date_idx ON public.posts USING btree (owneruserid ASC, creationdate);

CREATE INDEX posts_owner_user_id_idx ON public.posts USING hash (owneruserid);

CREATE INDEX posts_parent_id_idx ON public.posts USING btree (parentid ASC);

CREATE INDEX posts_post_type_id_idx ON public.posts USING btree (posttypeid ASC);

CREATE INDEX posts_score_idx ON public.posts USING btree (score ASC);

CREATE INDEX posts_viewcount_idx ON public.posts USING btree (viewcount ASC);

CREATE INDEX posttags_postid_idx ON public.posttags USING hash (postid);

CREATE INDEX posttags_tagid_idx ON public.posttags USING btree (tagid ASC);

CREATE INDEX tags_count_idx ON public.tags USING btree (count ASC);

CREATE INDEX tags_name_idx ON public.tags USING hash (tagname);

CREATE INDEX user_acc_id_idx ON public.users USING hash (accountid);

CREATE INDEX user_created_at_idx ON public.users USING btree (creationdate ASC);

CREATE INDEX user_display_idx ON public.users USING hash (displayname);

CREATE INDEX user_down_votes_idx ON public.users USING btree (downvotes ASC);

CREATE INDEX user_up_votes_idx ON public.users USING btree (upvotes ASC);

CREATE INDEX usertagqa_all_qa_posts_idx ON public.usertagqa USING btree ((questions + answers) ASC);

CREATE INDEX usertagqa_answers_idx ON public.usertagqa USING btree (answers ASC);

CREATE INDEX usertagqa_questions_answers_idx ON public.usertagqa USING btree (questions ASC, answers);

CREATE INDEX votes_creation_date_idx ON public.votes USING btree (creationdate ASC);

CREATE INDEX votes_post_id_idx ON public.votes USING hash (postid);

CREATE INDEX votes_type_idx ON public.votes USING btree (votetypeid ASC);
--
-- PostgreSQL database dump
--

-- Dumped from database version 12.14
-- Dumped by pg_dump version 14.12

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: public; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA public;


--
-- Name: tsm_system_rows; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS tsm_system_rows WITH SCHEMA public;


SET default_table_access_method = heap;

--
-- Name: allposttags; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.allposttags (
    postid integer NOT NULL,
    tagid integer NOT NULL
);


--
-- Name: posts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.posts (
    id integer NOT NULL,
    posttypeid integer NOT NULL,
    acceptedanswerid integer,
    parentid integer,
    creationdate timestamp without time zone NOT NULL,
    score integer,
    viewcount integer,
    body text,
    owneruserid integer,
    lasteditoruserid integer,
    lasteditordisplayname text,
    lasteditdate timestamp without time zone,
    lastactivitydate timestamp without time zone,
    title text,
    tags text,
    answercount integer,
    commentcount integer,
    favoritecount integer,
    closeddate timestamp without time zone,
    communityowneddate timestamp without time zone,
    jsonfield jsonb
);


--
-- Name: answers; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.answers AS
 SELECT posts.id,
    posts.parentid,
    posts.creationdate,
    posts.score,
    posts.owneruserid,
    posts.lasteditoruserid,
    posts.lasteditordisplayname,
    posts.lasteditdate,
    posts.lastactivitydate,
    posts.commentcount,
    posts.communityowneddate
   FROM public.posts
  WHERE (posts.posttypeid = 2);


--
-- Name: badges; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.badges (
    id integer NOT NULL,
    userid integer NOT NULL,
    name text NOT NULL,
    date timestamp without time zone NOT NULL,
    jsonfield jsonb
);


--
-- Name: closeasofftopicreasontypes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.closeasofftopicreasontypes (
    id integer NOT NULL,
    isuniversal boolean NOT NULL,
    markdownmini text NOT NULL,
    creationdate timestamp without time zone,
    creationmoderatorid integer,
    approvaldate timestamp without time zone,
    approvalmoderatorid integer,
    deactivationdate timestamp without time zone,
    deactivationmoderatorid integer
);


--
-- Name: closereasontypes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.closereasontypes (
    id integer NOT NULL,
    name text NOT NULL,
    description text
);


--
-- Name: comments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.comments (
    id integer NOT NULL,
    postid integer NOT NULL,
    score integer NOT NULL,
    text text,
    creationdate timestamp without time zone NOT NULL,
    userid integer,
    jsonfield jsonb
);


--
-- Name: flagtypes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.flagtypes (
    id integer NOT NULL,
    name text NOT NULL,
    description text NOT NULL
);


--
-- Name: posthistory; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.posthistory (
    id integer NOT NULL,
    posthistorytypeid integer,
    postid integer,
    revisionguid text,
    creationdate timestamp without time zone NOT NULL,
    userid integer,
    posttext text,
    jsonfield jsonb
);


--
-- Name: posthistorytypes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.posthistorytypes (
    id integer NOT NULL,
    name text NOT NULL
);


--
-- Name: postlinks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.postlinks (
    id integer NOT NULL,
    creationdate timestamp without time zone NOT NULL,
    postid integer NOT NULL,
    relatedpostid integer NOT NULL,
    linktypeid integer NOT NULL,
    jsonfield jsonb
);


--
-- Name: postlinktypes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.postlinktypes (
    id integer NOT NULL,
    name text
);


--
-- Name: posttags; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.posttags (
    postid integer NOT NULL,
    tagid integer NOT NULL
);


--
-- Name: posttypes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.posttypes (
    id integer NOT NULL,
    name text NOT NULL
);


--
-- Name: questionanswer; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.questionanswer (
    questionid integer NOT NULL,
    answerid integer NOT NULL
);


--
-- Name: questions; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.questions AS
 SELECT posts.id,
    posts.acceptedanswerid,
    posts.creationdate,
    posts.score,
    posts.viewcount,
    posts.owneruserid,
    posts.lasteditoruserid,
    posts.lasteditordisplayname,
    posts.lasteditdate,
    posts.lastactivitydate,
    posts.title,
    posts.tags,
    posts.answercount,
    posts.commentcount,
    posts.favoritecount,
    posts.communityowneddate
   FROM public.posts
  WHERE (posts.posttypeid = 1);


--
-- Name: reviewtaskresulttype; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.reviewtaskresulttype (
    id integer NOT NULL,
    name text,
    description text
);


--
-- Name: reviewtasktypes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.reviewtasktypes (
    id integer NOT NULL,
    name text,
    description text
);


--
-- Name: tags; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tags (
    id integer NOT NULL,
    tagname text NOT NULL,
    count integer,
    excerptpostid integer,
    wikipostid integer,
    jsonfield jsonb
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id integer NOT NULL,
    reputation integer NOT NULL,
    creationdate timestamp without time zone NOT NULL,
    displayname character varying(40) NOT NULL,
    lastaccessdate timestamp without time zone,
    websiteurl text,
    location text,
    aboutme text,
    views integer NOT NULL,
    upvotes integer NOT NULL,
    downvotes integer NOT NULL,
    profileimageurl text,
    age integer,
    accountid integer,
    jsonfield jsonb
);


--
-- Name: usertagqa; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.usertagqa (
    userid integer NOT NULL,
    tagid integer NOT NULL,
    questions integer,
    answers integer
);


--
-- Name: votes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.votes (
    id integer NOT NULL,
    postid integer,
    votetypeid integer NOT NULL,
    userid integer,
    creationdate timestamp without time zone NOT NULL,
    bountyamount integer,
    jsonfield jsonb
);


--
-- Name: votetypes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.votetypes (
    id integer NOT NULL,
    name text
);


--
-- Name: allposttags allposttags_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.allposttags
    ADD CONSTRAINT allposttags_pkey PRIMARY KEY (postid, tagid);


--
-- Name: badges badges_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.badges
    ADD CONSTRAINT badges_pkey PRIMARY KEY (id);


--
-- Name: closeasofftopicreasontypes closeasofftopicreasontypes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.closeasofftopicreasontypes
    ADD CONSTRAINT closeasofftopicreasontypes_pkey PRIMARY KEY (id);


--
-- Name: closereasontypes closereasontypes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.closereasontypes
    ADD CONSTRAINT closereasontypes_pkey PRIMARY KEY (id);


--
-- Name: comments comments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comments
    ADD CONSTRAINT comments_pkey PRIMARY KEY (id);


--
-- Name: flagtypes flagtypes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.flagtypes
    ADD CONSTRAINT flagtypes_pkey PRIMARY KEY (id);


--
-- Name: posthistory posthistory_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.posthistory
    ADD CONSTRAINT posthistory_pkey PRIMARY KEY (id);


--
-- Name: posthistorytypes posthistorytypes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.posthistorytypes
    ADD CONSTRAINT posthistorytypes_pkey PRIMARY KEY (id);


--
-- Name: postlinks postlinks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.postlinks
    ADD CONSTRAINT postlinks_pkey PRIMARY KEY (id);


--
-- Name: postlinktypes postlinktypes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.postlinktypes
    ADD CONSTRAINT postlinktypes_pkey PRIMARY KEY (id);


--
-- Name: posts posts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.posts
    ADD CONSTRAINT posts_pkey PRIMARY KEY (id);


--
-- Name: posttags posttags_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.posttags
    ADD CONSTRAINT posttags_pkey PRIMARY KEY (postid, tagid);


--
-- Name: posttypes posttypes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.posttypes
    ADD CONSTRAINT posttypes_pkey PRIMARY KEY (id);


--
-- Name: questionanswer questionanswer_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.questionanswer
    ADD CONSTRAINT questionanswer_pkey PRIMARY KEY (questionid, answerid);


--
-- Name: reviewtaskresulttype reviewtaskresulttype_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviewtaskresulttype
    ADD CONSTRAINT reviewtaskresulttype_pkey PRIMARY KEY (id);


--
-- Name: reviewtasktypes reviewtasktypes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviewtasktypes
    ADD CONSTRAINT reviewtasktypes_pkey PRIMARY KEY (id);


--
-- Name: tags tags_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tags
    ADD CONSTRAINT tags_pkey PRIMARY KEY (id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: usertagqa usertagqa_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.usertagqa
    ADD CONSTRAINT usertagqa_pkey PRIMARY KEY (userid, tagid);


--
-- Name: votes votes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.votes
    ADD CONSTRAINT votes_pkey PRIMARY KEY (id);


--
-- Name: votetypes votetypes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.votetypes
    ADD CONSTRAINT votetypes_pkey PRIMARY KEY (id);


--
-- Name: badges_date_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX badges_date_idx ON public.badges USING btree (date) WITH (fillfactor='100');


--
-- Name: badges_name_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX badges_name_idx ON public.badges USING btree (name) WITH (fillfactor='100');


--
-- Name: badges_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX badges_user_id_idx ON public.badges USING btree (userid) WITH (fillfactor='100');


--
-- Name: cmnts_creation_date_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX cmnts_creation_date_idx ON public.comments USING btree (creationdate) WITH (fillfactor='100');


--
-- Name: cmnts_postid_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX cmnts_postid_idx ON public.comments USING hash (postid) WITH (fillfactor='100');


--
-- Name: cmnts_score_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX cmnts_score_idx ON public.comments USING btree (score) WITH (fillfactor='100');


--
-- Name: cmnts_userid_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX cmnts_userid_idx ON public.comments USING btree (userid) WITH (fillfactor='100');


--
-- Name: ph_creation_date_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ph_creation_date_idx ON public.posthistory USING btree (creationdate) WITH (fillfactor='100');


--
-- Name: ph_post_type_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ph_post_type_id_idx ON public.posthistory USING btree (posthistorytypeid) WITH (fillfactor='100');


--
-- Name: ph_postid_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ph_postid_idx ON public.posthistory USING hash (postid) WITH (fillfactor='100');


--
-- Name: ph_revguid_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ph_revguid_idx ON public.posthistory USING btree (revisionguid) WITH (fillfactor='100');


--
-- Name: ph_userid_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ph_userid_idx ON public.posthistory USING btree (userid) WITH (fillfactor='100');


--
-- Name: postlinks_post_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX postlinks_post_id_idx ON public.postlinks USING btree (postid) WITH (fillfactor='100');


--
-- Name: postlinks_related_post_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX postlinks_related_post_id_idx ON public.postlinks USING btree (relatedpostid) WITH (fillfactor='100');


--
-- Name: posts_accepted_answer_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX posts_accepted_answer_id_idx ON public.posts USING btree (acceptedanswerid) WITH (fillfactor='100');


--
-- Name: posts_answer_count_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX posts_answer_count_idx ON public.posts USING btree (answercount) WITH (fillfactor='100');


--
-- Name: posts_comment_count_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX posts_comment_count_idx ON public.posts USING btree (commentcount) WITH (fillfactor='100');


--
-- Name: posts_creation_date_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX posts_creation_date_idx ON public.posts USING btree (creationdate) WITH (fillfactor='100');


--
-- Name: posts_favorite_count_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX posts_favorite_count_idx ON public.posts USING btree (favoritecount) WITH (fillfactor='100');


--
-- Name: posts_id_accepted_answers_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX posts_id_accepted_answers_id_idx ON public.posts USING btree (id, acceptedanswerid) WITH (fillfactor='100');


--
-- Name: posts_id_parent_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX posts_id_parent_id_idx ON public.posts USING btree (id, parentid) WITH (fillfactor='100');


--
-- Name: posts_id_post_type_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX posts_id_post_type_id_idx ON public.posts USING btree (id, posttypeid) WITH (fillfactor='100');


--
-- Name: posts_owner_user_id_creation_date_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX posts_owner_user_id_creation_date_idx ON public.posts USING btree (owneruserid, creationdate) WITH (fillfactor='100');


--
-- Name: posts_owner_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX posts_owner_user_id_idx ON public.posts USING hash (owneruserid) WITH (fillfactor='100');


--
-- Name: posts_parent_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX posts_parent_id_idx ON public.posts USING btree (parentid) WITH (fillfactor='100');


--
-- Name: posts_post_type_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX posts_post_type_id_idx ON public.posts USING btree (posttypeid) WITH (fillfactor='100');


--
-- Name: posts_score_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX posts_score_idx ON public.posts USING btree (score) WITH (fillfactor='100');


--
-- Name: posts_viewcount_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX posts_viewcount_idx ON public.posts USING btree (viewcount) WITH (fillfactor='100');


--
-- Name: posttags_postid_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX posttags_postid_idx ON public.posttags USING hash (postid) WITH (fillfactor='100');


--
-- Name: posttags_tagid_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX posttags_tagid_idx ON public.posttags USING btree (tagid) WITH (fillfactor='100');


--
-- Name: tags_count_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tags_count_idx ON public.tags USING btree (count) WITH (fillfactor='100');


--
-- Name: tags_name_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tags_name_idx ON public.tags USING hash (tagname) WITH (fillfactor='100');


--
-- Name: user_acc_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_acc_id_idx ON public.users USING hash (accountid) WITH (fillfactor='100');


--
-- Name: user_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_created_at_idx ON public.users USING btree (creationdate) WITH (fillfactor='100');


--
-- Name: user_display_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_display_idx ON public.users USING hash (displayname) WITH (fillfactor='100');


--
-- Name: user_down_votes_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_down_votes_idx ON public.users USING btree (downvotes) WITH (fillfactor='100');


--
-- Name: user_up_votes_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_up_votes_idx ON public.users USING btree (upvotes) WITH (fillfactor='100');


--
-- Name: usertagqa_all_qa_posts_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX usertagqa_all_qa_posts_idx ON public.usertagqa USING btree (((questions + answers))) WITH (fillfactor='100');


--
-- Name: usertagqa_answers_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX usertagqa_answers_idx ON public.usertagqa USING btree (answers) WITH (fillfactor='100');


--
-- Name: usertagqa_questions_answers_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX usertagqa_questions_answers_idx ON public.usertagqa USING btree (questions, answers) WITH (fillfactor='100');


--
-- Name: usertagqa_questions_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX usertagqa_questions_idx ON public.usertagqa USING btree (questions) WITH (fillfactor='100');


--
-- Name: votes_creation_date_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX votes_creation_date_idx ON public.votes USING btree (creationdate) WITH (fillfactor='100');


--
-- Name: votes_post_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX votes_post_id_idx ON public.votes USING hash (postid) WITH (fillfactor='100');


--
-- Name: votes_type_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX votes_type_idx ON public.votes USING btree (votetypeid) WITH (fillfactor='100');


--
-- Name: badges fk_badges_userid; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.badges
    ADD CONSTRAINT fk_badges_userid FOREIGN KEY (userid) REFERENCES public.users(id);


--
-- Name: comments fk_comments_postid; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comments
    ADD CONSTRAINT fk_comments_postid FOREIGN KEY (postid) REFERENCES public.posts(id);


--
-- Name: comments fk_comments_userid; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comments
    ADD CONSTRAINT fk_comments_userid FOREIGN KEY (userid) REFERENCES public.users(id);


--
-- Name: posthistory fk_posthistory_postid; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.posthistory
    ADD CONSTRAINT fk_posthistory_postid FOREIGN KEY (postid) REFERENCES public.posts(id);


--
-- Name: posthistory fk_posthistory_userid; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.posthistory
    ADD CONSTRAINT fk_posthistory_userid FOREIGN KEY (userid) REFERENCES public.users(id);


--
-- Name: postlinks fk_postlinks_postid; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.postlinks
    ADD CONSTRAINT fk_postlinks_postid FOREIGN KEY (postid) REFERENCES public.posts(id) NOT VALID;


--
-- Name: postlinks fk_postlinks_relatedpostid; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.postlinks
    ADD CONSTRAINT fk_postlinks_relatedpostid FOREIGN KEY (relatedpostid) REFERENCES public.posts(id) NOT VALID;


--
-- Name: posts fk_posts_lasteditoruserid; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.posts
    ADD CONSTRAINT fk_posts_lasteditoruserid FOREIGN KEY (lasteditoruserid) REFERENCES public.users(id);


--
-- Name: posts fk_posts_owneruserid; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.posts
    ADD CONSTRAINT fk_posts_owneruserid FOREIGN KEY (owneruserid) REFERENCES public.users(id);


--
-- Name: posts fk_posts_parentid; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.posts
    ADD CONSTRAINT fk_posts_parentid FOREIGN KEY (parentid) REFERENCES public.posts(id);


--
-- Name: votes fk_votes_postid; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.votes
    ADD CONSTRAINT fk_votes_postid FOREIGN KEY (postid) REFERENCES public.posts(id) NOT VALID;


--
-- Name: votes fk_votes_userid; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.votes
    ADD CONSTRAINT fk_votes_userid FOREIGN KEY (userid) REFERENCES public.users(id);


--
-- PostgreSQL database dump complete
--


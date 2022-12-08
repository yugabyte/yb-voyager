CREATE INDEX film_fulltext_idx ON public.film USING gist (fulltext);

CREATE INDEX idx_actor_last_name ON public.actor USING btree (last_name);

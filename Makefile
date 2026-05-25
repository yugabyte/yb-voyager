.PHONY: pg-yb-parser-rebuild pg-yb-parser-check

# Rebuild the YB-extended SQL parser end-to-end.
#
# Fetches upstream libpg_query, applies our YB-EXT patches, regenerates the
# pg_query_go snapshot under third_party/pg_query_go/, restores the committed
# YB-EXT deparse edits, then rebuilds voyager. See third_party/POC.md and
# third_party/pg-yb-parser/README.md for the workflow.
pg-yb-parser-rebuild:
	./third_party/pg-yb-parser/regen.sh
	cd yb-voyager && go build ./...

# Smoke-check that the parser is wired into voyager correctly. Just builds
# and runs the parser-adjacent unit tests against the committed pg_query_go
# snapshot. Does not regenerate anything.
pg-yb-parser-check:
	cd yb-voyager && go build ./...
	cd yb-voyager && go test -tags unit -count=1 \
		./src/query/queryparser/... \
		./src/query/queryissue/... \
		./src/query/sqltransformer/...

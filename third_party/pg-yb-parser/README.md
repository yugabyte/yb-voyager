# pg-yb-parser — YB-EXT delta for the SQL parser

This directory holds the **modifications** voyager makes to upstream
`libpg_query` so that the SQL parser accepts YugabyteDB-specific grammar
extensions. The upstream `libpg_query` is **not vendored** — it's fetched
fresh from GitHub at regen time. Only our delta lives in git.

## Layout

```
pg-yb-parser/
├── README.md               (this file)
├── NOTICE                  third-party attribution
├── Dockerfile.regen        Ubuntu 24.04 + bison + ffi-clang + protoc
├── regen.sh                fetch upstream → apply patches → regen → snapshot
├── 00_yb_toolchain.patch   one-time portability fixes for libpg_query itself
│                              (Makefile sed/LIBCLANG/regen_proto, extract_source.rb)
└── features/
    └── 99_yb_sortby_hash.patch    one patch per YB feature
                                   (touches gram.y, parsenodes.h, kwlist.h,
                                   srcdata/enum_defs.json)
```

Outside this dir, our YB-EXT delta also includes a small fenced edit in
the committed `../pg_query_go/parser/postgres_deparse.c` (the AST→SQL emit
cases for each feature, marked with `/* YB-EXT BEGIN: <feature> */`).

## How a YB feature lands end-to-end

```
1. Edit features/99_yb_<feature>.patch                  ← grammar + AST + srcdata
2. Edit ../pg_query_go/parser/postgres_deparse.c        ← deparse emit case
3. ./regen.sh                                            ← rebuilds the snapshot
4. cd ../../yb-voyager && go test -tags unit ./src/query/...
```

`regen.sh` does:

1. `git clone` upstream libpg_query at the pinned tag
2. Apply `00_yb_toolchain.patch` to that tree (Makefile / extract_source.rb fixes)
3. Copy `features/*.patch` into the tree's `patches/` so `make extract_source` applies them
4. Run `make extract_source && make regen_proto` inside Docker — generates fresh `gram.c`, `kwlist_d.h`, `pg_query.proto`, `pg_query_enum_defs.c`, `pg_query.pb-c.{c,h}`
5. `cd ../pg_query_go && make update_source` — snapshots into the committed Go module, regen `pg_query.pb.go`
6. `git checkout HEAD -- ../pg_query_go/parser/postgres_deparse.c` — restores our committed YB-EXT deparse cases that step 5 clobbered

## Upgrading to a new upstream libpg_query

```sh
LIB_TAG=18-1.0.0 ./regen.sh
```

If any patch fails to apply against the new upstream `gram.y` /
`parsenodes.h`, `regen.sh` exits with the conflict path; re-author the
affected hunks against the new context.

## Prereqs

- Docker (any platform)
- `go` available locally (for the `pg_config.h` macOS-restore step that
  uses `go env GOMODCACHE`)

No Ruby, bison, libclang, or protoc needed on the host — they're all
inside the Docker image.

## See also

- `../POC.md` — full proof-of-concept writeup
- `../pg_query_go/` — the committed snapshot that voyager builds against
- `../../yb-voyager/src/query/queryparser/yb_hash_test.go` — example test

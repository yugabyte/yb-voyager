/* A Bison parser, made by GNU Bison 3.8.2.  */

/* Bison interface for Yacc-like parsers in C

   Copyright (C) 1984, 1989-1990, 2000-2015, 2018-2021 Free Software Foundation,
   Inc.

   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with this program.  If not, see <https://www.gnu.org/licenses/>.  */

/* As a special exception, you may create a larger work that contains
   part or all of the Bison parser skeleton and distribute that work
   under terms of your choice, so long as that work isn't itself a
   parser generator using the skeleton or a modified version thereof
   as a parser skeleton.  Alternatively, if you modify or redistribute
   the parser skeleton itself, you may (at your option) remove this
   special exception, which will cause the skeleton and the resulting
   Bison output files to be licensed under the GNU General Public
   License without this special exception.

   This special exception was added by the Free Software Foundation in
   version 2.2 of Bison.  */

/* DO NOT RELY ON FEATURES THAT ARE NOT DOCUMENTED in the manual,
   especially those whose name start with YY_ or yy_.  They are
   private implementation details that can be changed or removed.  */

#ifndef YY_BASE_YY_GRAM_H_INCLUDED
# define YY_BASE_YY_GRAM_H_INCLUDED
/* Debug traces.  */
#ifndef YYDEBUG
# define YYDEBUG 0
#endif
#if YYDEBUG
extern int base_yydebug;
#endif

/* Token kinds.  */
#ifndef YYTOKENTYPE
# define YYTOKENTYPE
  enum yytokentype
  {
    YYEMPTY = -2,
    YYEOF = 0,                     /* "end of file"  */
    YYerror = 256,                 /* error  */
    YYUNDEF = 257,                 /* "invalid token"  */
    IDENT = 258,                   /* IDENT  */
    UIDENT = 259,                  /* UIDENT  */
    FCONST = 260,                  /* FCONST  */
    SCONST = 261,                  /* SCONST  */
    USCONST = 262,                 /* USCONST  */
    BCONST = 263,                  /* BCONST  */
    XCONST = 264,                  /* XCONST  */
    Op = 265,                      /* Op  */
    ICONST = 266,                  /* ICONST  */
    PARAM = 267,                   /* PARAM  */
    TYPECAST = 268,                /* TYPECAST  */
    DOT_DOT = 269,                 /* DOT_DOT  */
    COLON_EQUALS = 270,            /* COLON_EQUALS  */
    EQUALS_GREATER = 271,          /* EQUALS_GREATER  */
    LESS_EQUALS = 272,             /* LESS_EQUALS  */
    GREATER_EQUALS = 273,          /* GREATER_EQUALS  */
    NOT_EQUALS = 274,              /* NOT_EQUALS  */
    SQL_COMMENT = 275,             /* SQL_COMMENT  */
    C_COMMENT = 276,               /* C_COMMENT  */
    ABORT_P = 277,                 /* ABORT_P  */
    ABSENT = 278,                  /* ABSENT  */
    ABSOLUTE_P = 279,              /* ABSOLUTE_P  */
    ACCESS = 280,                  /* ACCESS  */
    ACTION = 281,                  /* ACTION  */
    ADD_P = 282,                   /* ADD_P  */
    ADMIN = 283,                   /* ADMIN  */
    AFTER = 284,                   /* AFTER  */
    AGGREGATE = 285,               /* AGGREGATE  */
    ALL = 286,                     /* ALL  */
    ALSO = 287,                    /* ALSO  */
    ALTER = 288,                   /* ALTER  */
    ALWAYS = 289,                  /* ALWAYS  */
    ANALYSE = 290,                 /* ANALYSE  */
    ANALYZE = 291,                 /* ANALYZE  */
    AND = 292,                     /* AND  */
    ANY = 293,                     /* ANY  */
    ARRAY = 294,                   /* ARRAY  */
    AS = 295,                      /* AS  */
    ASC = 296,                     /* ASC  */
    ASENSITIVE = 297,              /* ASENSITIVE  */
    ASSERTION = 298,               /* ASSERTION  */
    ASSIGNMENT = 299,              /* ASSIGNMENT  */
    ASYMMETRIC = 300,              /* ASYMMETRIC  */
    ATOMIC = 301,                  /* ATOMIC  */
    AT = 302,                      /* AT  */
    ATTACH = 303,                  /* ATTACH  */
    ATTRIBUTE = 304,               /* ATTRIBUTE  */
    AUTHORIZATION = 305,           /* AUTHORIZATION  */
    BACKWARD = 306,                /* BACKWARD  */
    BEFORE = 307,                  /* BEFORE  */
    BEGIN_P = 308,                 /* BEGIN_P  */
    BETWEEN = 309,                 /* BETWEEN  */
    BIGINT = 310,                  /* BIGINT  */
    BINARY = 311,                  /* BINARY  */
    BIT = 312,                     /* BIT  */
    BOOLEAN_P = 313,               /* BOOLEAN_P  */
    BOTH = 314,                    /* BOTH  */
    BREADTH = 315,                 /* BREADTH  */
    BY = 316,                      /* BY  */
    CACHE = 317,                   /* CACHE  */
    CALL = 318,                    /* CALL  */
    CALLED = 319,                  /* CALLED  */
    CASCADE = 320,                 /* CASCADE  */
    CASCADED = 321,                /* CASCADED  */
    CASE = 322,                    /* CASE  */
    CAST = 323,                    /* CAST  */
    CATALOG_P = 324,               /* CATALOG_P  */
    CHAIN = 325,                   /* CHAIN  */
    CHAR_P = 326,                  /* CHAR_P  */
    CHARACTER = 327,               /* CHARACTER  */
    CHARACTERISTICS = 328,         /* CHARACTERISTICS  */
    CHECK = 329,                   /* CHECK  */
    CHECKPOINT = 330,              /* CHECKPOINT  */
    CLASS = 331,                   /* CLASS  */
    CLOSE = 332,                   /* CLOSE  */
    CLUSTER = 333,                 /* CLUSTER  */
    COALESCE = 334,                /* COALESCE  */
    COLLATE = 335,                 /* COLLATE  */
    COLLATION = 336,               /* COLLATION  */
    COLUMN = 337,                  /* COLUMN  */
    COLUMNS = 338,                 /* COLUMNS  */
    COMMENT = 339,                 /* COMMENT  */
    COMMENTS = 340,                /* COMMENTS  */
    COMMIT = 341,                  /* COMMIT  */
    COMMITTED = 342,               /* COMMITTED  */
    COMPRESSION = 343,             /* COMPRESSION  */
    CONCURRENTLY = 344,            /* CONCURRENTLY  */
    CONDITIONAL = 345,             /* CONDITIONAL  */
    CONFIGURATION = 346,           /* CONFIGURATION  */
    CONFLICT = 347,                /* CONFLICT  */
    CONNECTION = 348,              /* CONNECTION  */
    CONSTRAINT = 349,              /* CONSTRAINT  */
    CONSTRAINTS = 350,             /* CONSTRAINTS  */
    CONTENT_P = 351,               /* CONTENT_P  */
    CONTINUE_P = 352,              /* CONTINUE_P  */
    CONVERSION_P = 353,            /* CONVERSION_P  */
    COPY = 354,                    /* COPY  */
    COST = 355,                    /* COST  */
    CREATE = 356,                  /* CREATE  */
    CROSS = 357,                   /* CROSS  */
    CSV = 358,                     /* CSV  */
    CUBE = 359,                    /* CUBE  */
    CURRENT_P = 360,               /* CURRENT_P  */
    CURRENT_CATALOG = 361,         /* CURRENT_CATALOG  */
    CURRENT_DATE = 362,            /* CURRENT_DATE  */
    CURRENT_ROLE = 363,            /* CURRENT_ROLE  */
    CURRENT_SCHEMA = 364,          /* CURRENT_SCHEMA  */
    CURRENT_TIME = 365,            /* CURRENT_TIME  */
    CURRENT_TIMESTAMP = 366,       /* CURRENT_TIMESTAMP  */
    CURRENT_USER = 367,            /* CURRENT_USER  */
    CURSOR = 368,                  /* CURSOR  */
    CYCLE = 369,                   /* CYCLE  */
    DATA_P = 370,                  /* DATA_P  */
    DATABASE = 371,                /* DATABASE  */
    DAY_P = 372,                   /* DAY_P  */
    DEALLOCATE = 373,              /* DEALLOCATE  */
    DEC = 374,                     /* DEC  */
    DECIMAL_P = 375,               /* DECIMAL_P  */
    DECLARE = 376,                 /* DECLARE  */
    DEFAULT = 377,                 /* DEFAULT  */
    DEFAULTS = 378,                /* DEFAULTS  */
    DEFERRABLE = 379,              /* DEFERRABLE  */
    DEFERRED = 380,                /* DEFERRED  */
    DEFINER = 381,                 /* DEFINER  */
    DELETE_P = 382,                /* DELETE_P  */
    DELIMITER = 383,               /* DELIMITER  */
    DELIMITERS = 384,              /* DELIMITERS  */
    DEPENDS = 385,                 /* DEPENDS  */
    DEPTH = 386,                   /* DEPTH  */
    DESC = 387,                    /* DESC  */
    DETACH = 388,                  /* DETACH  */
    DICTIONARY = 389,              /* DICTIONARY  */
    DISABLE_P = 390,               /* DISABLE_P  */
    DISCARD = 391,                 /* DISCARD  */
    DISTINCT = 392,                /* DISTINCT  */
    DO = 393,                      /* DO  */
    DOCUMENT_P = 394,              /* DOCUMENT_P  */
    DOMAIN_P = 395,                /* DOMAIN_P  */
    DOUBLE_P = 396,                /* DOUBLE_P  */
    DROP = 397,                    /* DROP  */
    EACH = 398,                    /* EACH  */
    ELSE = 399,                    /* ELSE  */
    EMPTY_P = 400,                 /* EMPTY_P  */
    ENABLE_P = 401,                /* ENABLE_P  */
    ENCODING = 402,                /* ENCODING  */
    ENCRYPTED = 403,               /* ENCRYPTED  */
    END_P = 404,                   /* END_P  */
    ENUM_P = 405,                  /* ENUM_P  */
    ERROR_P = 406,                 /* ERROR_P  */
    ESCAPE = 407,                  /* ESCAPE  */
    EVENT = 408,                   /* EVENT  */
    EXCEPT = 409,                  /* EXCEPT  */
    EXCLUDE = 410,                 /* EXCLUDE  */
    EXCLUDING = 411,               /* EXCLUDING  */
    EXCLUSIVE = 412,               /* EXCLUSIVE  */
    EXECUTE = 413,                 /* EXECUTE  */
    EXISTS = 414,                  /* EXISTS  */
    EXPLAIN = 415,                 /* EXPLAIN  */
    EXPRESSION = 416,              /* EXPRESSION  */
    EXTENSION = 417,               /* EXTENSION  */
    EXTERNAL = 418,                /* EXTERNAL  */
    EXTRACT = 419,                 /* EXTRACT  */
    FALSE_P = 420,                 /* FALSE_P  */
    FAMILY = 421,                  /* FAMILY  */
    FETCH = 422,                   /* FETCH  */
    FILTER = 423,                  /* FILTER  */
    FINALIZE = 424,                /* FINALIZE  */
    FIRST_P = 425,                 /* FIRST_P  */
    FLOAT_P = 426,                 /* FLOAT_P  */
    FOLLOWING = 427,               /* FOLLOWING  */
    FOR = 428,                     /* FOR  */
    FORCE = 429,                   /* FORCE  */
    FOREIGN = 430,                 /* FOREIGN  */
    FORMAT = 431,                  /* FORMAT  */
    FORWARD = 432,                 /* FORWARD  */
    FREEZE = 433,                  /* FREEZE  */
    FROM = 434,                    /* FROM  */
    FULL = 435,                    /* FULL  */
    FUNCTION = 436,                /* FUNCTION  */
    FUNCTIONS = 437,               /* FUNCTIONS  */
    GENERATED = 438,               /* GENERATED  */
    GLOBAL = 439,                  /* GLOBAL  */
    GRANT = 440,                   /* GRANT  */
    GRANTED = 441,                 /* GRANTED  */
    GREATEST = 442,                /* GREATEST  */
    GROUP_P = 443,                 /* GROUP_P  */
    GROUPING = 444,                /* GROUPING  */
    GROUPS = 445,                  /* GROUPS  */
    HANDLER = 446,                 /* HANDLER  */
    HASH = 447,                    /* HASH  */
    HAVING = 448,                  /* HAVING  */
    HEADER_P = 449,                /* HEADER_P  */
    HOLD = 450,                    /* HOLD  */
    HOUR_P = 451,                  /* HOUR_P  */
    IDENTITY_P = 452,              /* IDENTITY_P  */
    IF_P = 453,                    /* IF_P  */
    ILIKE = 454,                   /* ILIKE  */
    IMMEDIATE = 455,               /* IMMEDIATE  */
    IMMUTABLE = 456,               /* IMMUTABLE  */
    IMPLICIT_P = 457,              /* IMPLICIT_P  */
    IMPORT_P = 458,                /* IMPORT_P  */
    IN_P = 459,                    /* IN_P  */
    INCLUDE = 460,                 /* INCLUDE  */
    INCLUDING = 461,               /* INCLUDING  */
    INCREMENT = 462,               /* INCREMENT  */
    INDENT = 463,                  /* INDENT  */
    INDEX = 464,                   /* INDEX  */
    INDEXES = 465,                 /* INDEXES  */
    INHERIT = 466,                 /* INHERIT  */
    INHERITS = 467,                /* INHERITS  */
    INITIALLY = 468,               /* INITIALLY  */
    INLINE_P = 469,                /* INLINE_P  */
    INNER_P = 470,                 /* INNER_P  */
    INOUT = 471,                   /* INOUT  */
    INPUT_P = 472,                 /* INPUT_P  */
    INSENSITIVE = 473,             /* INSENSITIVE  */
    INSERT = 474,                  /* INSERT  */
    INSTEAD = 475,                 /* INSTEAD  */
    INT_P = 476,                   /* INT_P  */
    INTEGER = 477,                 /* INTEGER  */
    INTERSECT = 478,               /* INTERSECT  */
    INTERVAL = 479,                /* INTERVAL  */
    INTO = 480,                    /* INTO  */
    INVOKER = 481,                 /* INVOKER  */
    IS = 482,                      /* IS  */
    ISNULL = 483,                  /* ISNULL  */
    ISOLATION = 484,               /* ISOLATION  */
    JOIN = 485,                    /* JOIN  */
    JSON = 486,                    /* JSON  */
    JSON_ARRAY = 487,              /* JSON_ARRAY  */
    JSON_ARRAYAGG = 488,           /* JSON_ARRAYAGG  */
    JSON_EXISTS = 489,             /* JSON_EXISTS  */
    JSON_OBJECT = 490,             /* JSON_OBJECT  */
    JSON_OBJECTAGG = 491,          /* JSON_OBJECTAGG  */
    JSON_QUERY = 492,              /* JSON_QUERY  */
    JSON_SCALAR = 493,             /* JSON_SCALAR  */
    JSON_SERIALIZE = 494,          /* JSON_SERIALIZE  */
    JSON_TABLE = 495,              /* JSON_TABLE  */
    JSON_VALUE = 496,              /* JSON_VALUE  */
    KEEP = 497,                    /* KEEP  */
    KEY = 498,                     /* KEY  */
    KEYS = 499,                    /* KEYS  */
    LABEL = 500,                   /* LABEL  */
    LANGUAGE = 501,                /* LANGUAGE  */
    LARGE_P = 502,                 /* LARGE_P  */
    LAST_P = 503,                  /* LAST_P  */
    LATERAL_P = 504,               /* LATERAL_P  */
    LEADING = 505,                 /* LEADING  */
    LEAKPROOF = 506,               /* LEAKPROOF  */
    LEAST = 507,                   /* LEAST  */
    LEFT = 508,                    /* LEFT  */
    LEVEL = 509,                   /* LEVEL  */
    LIKE = 510,                    /* LIKE  */
    LIMIT = 511,                   /* LIMIT  */
    LISTEN = 512,                  /* LISTEN  */
    LOAD = 513,                    /* LOAD  */
    LOCAL = 514,                   /* LOCAL  */
    LOCALTIME = 515,               /* LOCALTIME  */
    LOCALTIMESTAMP = 516,          /* LOCALTIMESTAMP  */
    LOCATION = 517,                /* LOCATION  */
    LOCK_P = 518,                  /* LOCK_P  */
    LOCKED = 519,                  /* LOCKED  */
    LOGGED = 520,                  /* LOGGED  */
    MAPPING = 521,                 /* MAPPING  */
    MATCH = 522,                   /* MATCH  */
    MATCHED = 523,                 /* MATCHED  */
    MATERIALIZED = 524,            /* MATERIALIZED  */
    MAXVALUE = 525,                /* MAXVALUE  */
    MERGE = 526,                   /* MERGE  */
    MERGE_ACTION = 527,            /* MERGE_ACTION  */
    METHOD = 528,                  /* METHOD  */
    MINUTE_P = 529,                /* MINUTE_P  */
    MINVALUE = 530,                /* MINVALUE  */
    MODE = 531,                    /* MODE  */
    MONTH_P = 532,                 /* MONTH_P  */
    MOVE = 533,                    /* MOVE  */
    NAME_P = 534,                  /* NAME_P  */
    NAMES = 535,                   /* NAMES  */
    NATIONAL = 536,                /* NATIONAL  */
    NATURAL = 537,                 /* NATURAL  */
    NCHAR = 538,                   /* NCHAR  */
    NESTED = 539,                  /* NESTED  */
    NEW = 540,                     /* NEW  */
    NEXT = 541,                    /* NEXT  */
    NFC = 542,                     /* NFC  */
    NFD = 543,                     /* NFD  */
    NFKC = 544,                    /* NFKC  */
    NFKD = 545,                    /* NFKD  */
    NO = 546,                      /* NO  */
    NONE = 547,                    /* NONE  */
    NORMALIZE = 548,               /* NORMALIZE  */
    NORMALIZED = 549,              /* NORMALIZED  */
    NOT = 550,                     /* NOT  */
    NOTHING = 551,                 /* NOTHING  */
    NOTIFY = 552,                  /* NOTIFY  */
    NOTNULL = 553,                 /* NOTNULL  */
    NOWAIT = 554,                  /* NOWAIT  */
    NULL_P = 555,                  /* NULL_P  */
    NULLIF = 556,                  /* NULLIF  */
    NULLS_P = 557,                 /* NULLS_P  */
    NUMERIC = 558,                 /* NUMERIC  */
    OBJECT_P = 559,                /* OBJECT_P  */
    OF = 560,                      /* OF  */
    OFF = 561,                     /* OFF  */
    OFFSET = 562,                  /* OFFSET  */
    OIDS = 563,                    /* OIDS  */
    OLD = 564,                     /* OLD  */
    OMIT = 565,                    /* OMIT  */
    ON = 566,                      /* ON  */
    ONLY = 567,                    /* ONLY  */
    OPERATOR = 568,                /* OPERATOR  */
    OPTION = 569,                  /* OPTION  */
    OPTIONS = 570,                 /* OPTIONS  */
    OR = 571,                      /* OR  */
    ORDER = 572,                   /* ORDER  */
    ORDINALITY = 573,              /* ORDINALITY  */
    OTHERS = 574,                  /* OTHERS  */
    OUT_P = 575,                   /* OUT_P  */
    OUTER_P = 576,                 /* OUTER_P  */
    OVER = 577,                    /* OVER  */
    OVERLAPS = 578,                /* OVERLAPS  */
    OVERLAY = 579,                 /* OVERLAY  */
    OVERRIDING = 580,              /* OVERRIDING  */
    OWNED = 581,                   /* OWNED  */
    OWNER = 582,                   /* OWNER  */
    PARALLEL = 583,                /* PARALLEL  */
    PARAMETER = 584,               /* PARAMETER  */
    PARSER = 585,                  /* PARSER  */
    PARTIAL = 586,                 /* PARTIAL  */
    PARTITION = 587,               /* PARTITION  */
    PASSING = 588,                 /* PASSING  */
    PASSWORD = 589,                /* PASSWORD  */
    PATH = 590,                    /* PATH  */
    PLACING = 591,                 /* PLACING  */
    PLAN = 592,                    /* PLAN  */
    PLANS = 593,                   /* PLANS  */
    POLICY = 594,                  /* POLICY  */
    POSITION = 595,                /* POSITION  */
    PRECEDING = 596,               /* PRECEDING  */
    PRECISION = 597,               /* PRECISION  */
    PRESERVE = 598,                /* PRESERVE  */
    PREPARE = 599,                 /* PREPARE  */
    PREPARED = 600,                /* PREPARED  */
    PRIMARY = 601,                 /* PRIMARY  */
    PRIOR = 602,                   /* PRIOR  */
    PRIVILEGES = 603,              /* PRIVILEGES  */
    PROCEDURAL = 604,              /* PROCEDURAL  */
    PROCEDURE = 605,               /* PROCEDURE  */
    PROCEDURES = 606,              /* PROCEDURES  */
    PROGRAM = 607,                 /* PROGRAM  */
    PUBLICATION = 608,             /* PUBLICATION  */
    QUOTE = 609,                   /* QUOTE  */
    QUOTES = 610,                  /* QUOTES  */
    RANGE = 611,                   /* RANGE  */
    READ = 612,                    /* READ  */
    REAL = 613,                    /* REAL  */
    REASSIGN = 614,                /* REASSIGN  */
    RECHECK = 615,                 /* RECHECK  */
    RECURSIVE = 616,               /* RECURSIVE  */
    REF_P = 617,                   /* REF_P  */
    REFERENCES = 618,              /* REFERENCES  */
    REFERENCING = 619,             /* REFERENCING  */
    REFRESH = 620,                 /* REFRESH  */
    REINDEX = 621,                 /* REINDEX  */
    RELATIVE_P = 622,              /* RELATIVE_P  */
    RELEASE = 623,                 /* RELEASE  */
    RENAME = 624,                  /* RENAME  */
    REPEATABLE = 625,              /* REPEATABLE  */
    REPLACE = 626,                 /* REPLACE  */
    REPLICA = 627,                 /* REPLICA  */
    RESET = 628,                   /* RESET  */
    RESTART = 629,                 /* RESTART  */
    RESTRICT = 630,                /* RESTRICT  */
    RETURN = 631,                  /* RETURN  */
    RETURNING = 632,               /* RETURNING  */
    RETURNS = 633,                 /* RETURNS  */
    REVOKE = 634,                  /* REVOKE  */
    RIGHT = 635,                   /* RIGHT  */
    ROLE = 636,                    /* ROLE  */
    ROLLBACK = 637,                /* ROLLBACK  */
    ROLLUP = 638,                  /* ROLLUP  */
    ROUTINE = 639,                 /* ROUTINE  */
    ROUTINES = 640,                /* ROUTINES  */
    ROW = 641,                     /* ROW  */
    ROWS = 642,                    /* ROWS  */
    RULE = 643,                    /* RULE  */
    SAVEPOINT = 644,               /* SAVEPOINT  */
    SCALAR = 645,                  /* SCALAR  */
    SCHEMA = 646,                  /* SCHEMA  */
    SCHEMAS = 647,                 /* SCHEMAS  */
    SCROLL = 648,                  /* SCROLL  */
    SEARCH = 649,                  /* SEARCH  */
    SECOND_P = 650,                /* SECOND_P  */
    SECURITY = 651,                /* SECURITY  */
    SELECT = 652,                  /* SELECT  */
    SEQUENCE = 653,                /* SEQUENCE  */
    SEQUENCES = 654,               /* SEQUENCES  */
    SERIALIZABLE = 655,            /* SERIALIZABLE  */
    SERVER = 656,                  /* SERVER  */
    SESSION = 657,                 /* SESSION  */
    SESSION_USER = 658,            /* SESSION_USER  */
    SET = 659,                     /* SET  */
    SETS = 660,                    /* SETS  */
    SETOF = 661,                   /* SETOF  */
    SHARE = 662,                   /* SHARE  */
    SHOW = 663,                    /* SHOW  */
    SIMILAR = 664,                 /* SIMILAR  */
    SIMPLE = 665,                  /* SIMPLE  */
    SKIP = 666,                    /* SKIP  */
    SMALLINT = 667,                /* SMALLINT  */
    SNAPSHOT = 668,                /* SNAPSHOT  */
    SOME = 669,                    /* SOME  */
    SOURCE = 670,                  /* SOURCE  */
    SQL_P = 671,                   /* SQL_P  */
    STABLE = 672,                  /* STABLE  */
    STANDALONE_P = 673,            /* STANDALONE_P  */
    START = 674,                   /* START  */
    STATEMENT = 675,               /* STATEMENT  */
    STATISTICS = 676,              /* STATISTICS  */
    STDIN = 677,                   /* STDIN  */
    STDOUT = 678,                  /* STDOUT  */
    STORAGE = 679,                 /* STORAGE  */
    STORED = 680,                  /* STORED  */
    STRICT_P = 681,                /* STRICT_P  */
    STRING_P = 682,                /* STRING_P  */
    STRIP_P = 683,                 /* STRIP_P  */
    SUBSCRIPTION = 684,            /* SUBSCRIPTION  */
    SUBSTRING = 685,               /* SUBSTRING  */
    SUPPORT = 686,                 /* SUPPORT  */
    SYMMETRIC = 687,               /* SYMMETRIC  */
    SYSID = 688,                   /* SYSID  */
    SYSTEM_P = 689,                /* SYSTEM_P  */
    SYSTEM_USER = 690,             /* SYSTEM_USER  */
    TABLE = 691,                   /* TABLE  */
    TABLES = 692,                  /* TABLES  */
    TABLESAMPLE = 693,             /* TABLESAMPLE  */
    TABLESPACE = 694,              /* TABLESPACE  */
    TARGET = 695,                  /* TARGET  */
    TEMP = 696,                    /* TEMP  */
    TEMPLATE = 697,                /* TEMPLATE  */
    TEMPORARY = 698,               /* TEMPORARY  */
    TEXT_P = 699,                  /* TEXT_P  */
    THEN = 700,                    /* THEN  */
    TIES = 701,                    /* TIES  */
    TIME = 702,                    /* TIME  */
    TIMESTAMP = 703,               /* TIMESTAMP  */
    TO = 704,                      /* TO  */
    TRAILING = 705,                /* TRAILING  */
    TRANSACTION = 706,             /* TRANSACTION  */
    TRANSFORM = 707,               /* TRANSFORM  */
    TREAT = 708,                   /* TREAT  */
    TRIGGER = 709,                 /* TRIGGER  */
    TRIM = 710,                    /* TRIM  */
    TRUE_P = 711,                  /* TRUE_P  */
    TRUNCATE = 712,                /* TRUNCATE  */
    TRUSTED = 713,                 /* TRUSTED  */
    TYPE_P = 714,                  /* TYPE_P  */
    TYPES_P = 715,                 /* TYPES_P  */
    UESCAPE = 716,                 /* UESCAPE  */
    UNBOUNDED = 717,               /* UNBOUNDED  */
    UNCONDITIONAL = 718,           /* UNCONDITIONAL  */
    UNCOMMITTED = 719,             /* UNCOMMITTED  */
    UNENCRYPTED = 720,             /* UNENCRYPTED  */
    UNION = 721,                   /* UNION  */
    UNIQUE = 722,                  /* UNIQUE  */
    UNKNOWN = 723,                 /* UNKNOWN  */
    UNLISTEN = 724,                /* UNLISTEN  */
    UNLOGGED = 725,                /* UNLOGGED  */
    UNTIL = 726,                   /* UNTIL  */
    UPDATE = 727,                  /* UPDATE  */
    USER = 728,                    /* USER  */
    USING = 729,                   /* USING  */
    VACUUM = 730,                  /* VACUUM  */
    VALID = 731,                   /* VALID  */
    VALIDATE = 732,                /* VALIDATE  */
    VALIDATOR = 733,               /* VALIDATOR  */
    VALUE_P = 734,                 /* VALUE_P  */
    VALUES = 735,                  /* VALUES  */
    VARCHAR = 736,                 /* VARCHAR  */
    VARIADIC = 737,                /* VARIADIC  */
    VARYING = 738,                 /* VARYING  */
    VERBOSE = 739,                 /* VERBOSE  */
    VERSION_P = 740,               /* VERSION_P  */
    VIEW = 741,                    /* VIEW  */
    VIEWS = 742,                   /* VIEWS  */
    VOLATILE = 743,                /* VOLATILE  */
    WHEN = 744,                    /* WHEN  */
    WHERE = 745,                   /* WHERE  */
    WHITESPACE_P = 746,            /* WHITESPACE_P  */
    WINDOW = 747,                  /* WINDOW  */
    WITH = 748,                    /* WITH  */
    WITHIN = 749,                  /* WITHIN  */
    WITHOUT = 750,                 /* WITHOUT  */
    WORK = 751,                    /* WORK  */
    WRAPPER = 752,                 /* WRAPPER  */
    WRITE = 753,                   /* WRITE  */
    XML_P = 754,                   /* XML_P  */
    XMLATTRIBUTES = 755,           /* XMLATTRIBUTES  */
    XMLCONCAT = 756,               /* XMLCONCAT  */
    XMLELEMENT = 757,              /* XMLELEMENT  */
    XMLEXISTS = 758,               /* XMLEXISTS  */
    XMLFOREST = 759,               /* XMLFOREST  */
    XMLNAMESPACES = 760,           /* XMLNAMESPACES  */
    XMLPARSE = 761,                /* XMLPARSE  */
    XMLPI = 762,                   /* XMLPI  */
    XMLROOT = 763,                 /* XMLROOT  */
    XMLSERIALIZE = 764,            /* XMLSERIALIZE  */
    XMLTABLE = 765,                /* XMLTABLE  */
    YEAR_P = 766,                  /* YEAR_P  */
    YES_P = 767,                   /* YES_P  */
    ZONE = 768,                    /* ZONE  */
    FORMAT_LA = 769,               /* FORMAT_LA  */
    NOT_LA = 770,                  /* NOT_LA  */
    NULLS_LA = 771,                /* NULLS_LA  */
    WITH_LA = 772,                 /* WITH_LA  */
    WITHOUT_LA = 773,              /* WITHOUT_LA  */
    MODE_TYPE_NAME = 774,          /* MODE_TYPE_NAME  */
    MODE_PLPGSQL_EXPR = 775,       /* MODE_PLPGSQL_EXPR  */
    MODE_PLPGSQL_ASSIGN1 = 776,    /* MODE_PLPGSQL_ASSIGN1  */
    MODE_PLPGSQL_ASSIGN2 = 777,    /* MODE_PLPGSQL_ASSIGN2  */
    MODE_PLPGSQL_ASSIGN3 = 778,    /* MODE_PLPGSQL_ASSIGN3  */
    UMINUS = 779,                  /* UMINUS  */
    NO_OPCLASS = 780               /* NO_OPCLASS  */
  };
  typedef enum yytokentype yytoken_kind_t;
#endif

/* Value type.  */
#if ! defined YYSTYPE && ! defined YYSTYPE_IS_DECLARED
union YYSTYPE
{
#line 233 "gram.y"

	core_YYSTYPE core_yystype;
	/* these fields must match core_YYSTYPE: */
	int			ival;
	char	   *str;
	const char *keyword;

	char		chr;
	bool		boolean;
	JoinType	jtype;
	DropBehavior dbehavior;
	OnCommitAction oncommit;
	List	   *list;
	Node	   *node;
	ObjectType	objtype;
	TypeName   *typnam;
	FunctionParameter *fun_param;
	FunctionParameterMode fun_param_mode;
	ObjectWithArgs *objwithargs;
	DefElem	   *defelt;
	SortBy	   *sortby;
	WindowDef  *windef;
	JoinExpr   *jexpr;
	IndexElem  *ielem;
	StatsElem  *selem;
	Alias	   *alias;
	RangeVar   *range;
	IntoClause *into;
	WithClause *with;
	InferClause	*infer;
	OnConflictClause *onconflict;
	A_Indices  *aind;
	ResTarget  *target;
	struct PrivTarget *privtarget;
	AccessPriv *accesspriv;
	struct ImportQual *importqual;
	InsertStmt *istmt;
	VariableSetStmt *vsetstmt;
	PartitionElem *partelem;
	PartitionSpec *partspec;
	PartitionBoundSpec *partboundspec;
	RoleSpec   *rolespec;
	PublicationObjSpec *publicationobjectspec;
	struct SelectLimit *selectlimit;
	SetQuantifier setquantifier;
	struct GroupClause *groupclause;
	MergeMatchKind mergematch;
	MergeWhenClause *mergewhen;
	struct KeyActions *keyactions;
	struct KeyAction *keyaction;

#line 641 "gram.h"

};
typedef union YYSTYPE YYSTYPE;
# define YYSTYPE_IS_TRIVIAL 1
# define YYSTYPE_IS_DECLARED 1
#endif

/* Location type.  */
#if ! defined YYLTYPE && ! defined YYLTYPE_IS_DECLARED
typedef struct YYLTYPE YYLTYPE;
struct YYLTYPE
{
  int first_line;
  int first_column;
  int last_line;
  int last_column;
};
# define YYLTYPE_IS_DECLARED 1
# define YYLTYPE_IS_TRIVIAL 1
#endif




int base_yyparse (core_yyscan_t yyscanner);


#endif /* !YY_BASE_YY_GRAM_H_INCLUDED  */

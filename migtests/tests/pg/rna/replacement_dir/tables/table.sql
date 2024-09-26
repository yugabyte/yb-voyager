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


CREATE TABLE rnacen.auth_group (
    id integer NOT NULL,
    name character varying(80) NOT NULL
);


CREATE TABLE rnacen.auth_group_permissions (
    id integer NOT NULL,
    group_id integer NOT NULL,
    permission_id integer NOT NULL
);


CREATE TABLE rnacen.auth_permission (
    id integer NOT NULL,
    name character varying(255) NOT NULL,
    content_type_id integer NOT NULL,
    codename character varying(100) NOT NULL
);


CREATE TABLE rnacen.auth_user (
    id integer NOT NULL,
    password character varying(128) NOT NULL,
    last_login timestamp with time zone,
    is_superuser boolean NOT NULL,
    username character varying(30) NOT NULL,
    first_name character varying(30) NOT NULL,
    last_name character varying(30) NOT NULL,
    email character varying(254) NOT NULL,
    is_staff boolean NOT NULL,
    is_active boolean NOT NULL,
    date_joined timestamp with time zone NOT NULL
);


CREATE TABLE rnacen.auth_user_groups (
    id integer NOT NULL,
    user_id integer NOT NULL,
    group_id integer NOT NULL
);


CREATE TABLE rnacen.auth_user_user_permissions (
    id integer NOT NULL,
    user_id integer NOT NULL,
    permission_id integer NOT NULL
);


CREATE TABLE rnacen.bad_precompute (
    id bigint NOT NULL,
    urs_taxid text NOT NULL
);


CREATE TABLE rnacen.blog (
    id bigint NOT NULL,
    title character varying(1000) NOT NULL,
    content text NOT NULL,
    created date DEFAULT ('now'::text)::date,
    featured boolean DEFAULT false,
    release_image character varying(255)
);


CREATE TABLE rnacen.corsheaders_corsmodel (
    id integer NOT NULL,
    cors character varying(255) NOT NULL
);


CREATE TABLE rnacen.cpat_results (
    urs_taxid text NOT NULL,
    fickett_score double precision NOT NULL,
    hexamer_score double precision NOT NULL,
    coding_probability double precision NOT NULL,
    is_protein_coding boolean NOT NULL
);


CREATE TABLE rnacen.django_admin_log (
    id integer NOT NULL,
    action_time timestamp with time zone NOT NULL,
    object_id text,
    object_repr character varying(200) NOT NULL,
    action_flag smallint NOT NULL,
    change_message text NOT NULL,
    content_type_id integer,
    user_id integer NOT NULL,
    CONSTRAINT django_admin_log_action_flag_check CHECK ((action_flag >= 0))
);


CREATE TABLE rnacen.django_content_type (
    id integer NOT NULL,
    app_label character varying(100) NOT NULL,
    model character varying(100) NOT NULL
);


CREATE TABLE rnacen.django_migrations (
    id integer NOT NULL,
    app character varying(255) NOT NULL,
    name character varying(255) NOT NULL,
    applied timestamp with time zone NOT NULL
);


CREATE TABLE rnacen.django_session (
    session_key character varying(40) NOT NULL,
    session_data text NOT NULL,
    expire_date timestamp with time zone NOT NULL
);


CREATE TABLE rnacen.django_site (
    id integer NOT NULL,
    domain character varying(100) NOT NULL,
    name character varying(50) NOT NULL
);


CREATE TABLE rnacen.ensembl_assembly (
    assembly_id character varying(255) NOT NULL,
    assembly_full_name character varying(255) NOT NULL,
    gca_accession character varying(20),
    assembly_ucsc character varying(100),
    common_name character varying(255),
    taxid integer NOT NULL,
    ensembl_url character varying(100),
    division character varying(20),
    blat_mapping integer,
    example_chromosome character varying(40),
    example_end integer,
    example_start integer,
    subdomain character varying(100) NOT NULL,
    selected_genome boolean DEFAULT false NOT NULL
);


CREATE TABLE rnacen.ensembl_compara (
    id integer NOT NULL,
    ensembl_transcript_id text NOT NULL,
    urs_taxid text NOT NULL,
    homology_id integer NOT NULL
);


CREATE TABLE rnacen.ensembl_coordinate_systems (
    id integer NOT NULL,
    chromosome character varying(100) NOT NULL,
    assembly_id character varying(255) NOT NULL,
    coordinate_system text NOT NULL,
    is_reference boolean NOT NULL,
    karyotype_rank integer
);


CREATE TABLE rnacen.ensembl_import_tracking (
    id integer NOT NULL,
    database_name text NOT NULL,
    task_name text NOT NULL,
    was_imported boolean
);


CREATE TABLE rnacen.ensembl_karyotype (
    id integer NOT NULL,
    karyotype jsonb,
    assembly_id character varying(255) DEFAULT 'foobar'::character varying
);


CREATE TABLE rnacen.ensembl_pseudogene_exons (
    id bigint NOT NULL,
    region_id integer,
    exon_start integer NOT NULL,
    exon_stop integer NOT NULL
);


CREATE TABLE rnacen.ensembl_pseudogene_regions (
    id bigint NOT NULL,
    gene text NOT NULL,
    region_name text NOT NULL,
    chromosome text NOT NULL,
    strand integer NOT NULL,
    region_start integer NOT NULL,
    region_stop integer NOT NULL,
    assembly_id character varying(255),
    exon_count integer NOT NULL,
    CONSTRAINT ck_ensembl_pseduogene_regions__exon_count_size CHECK ((exon_count > 0))
);


CREATE TABLE rnacen.go_term_annotations (
    go_term_annotation_id integer NOT NULL,
    rna_id character varying(50) NOT NULL,
    qualifier text NOT NULL,
    ontology_term_id character varying(10) NOT NULL,
    evidence_code character varying(11) NOT NULL,
    assigned_by character varying(50),
    extensions jsonb
);


CREATE TABLE rnacen.go_term_publication_map (
    go_term_publication_mapping_id integer NOT NULL,
    go_term_annotation_id integer NOT NULL,
    reference_id bigint NOT NULL
);


CREATE TABLE rnacen.insdc_so_term_mapping (
    rna_type text NOT NULL,
    so_term_id text NOT NULL
);


CREATE TABLE rnacen.litscan_sentence_id_counts (
    sent_ids integer,
    id_count bigint
);


CREATE TABLE rnacen.litscan_statistics (
    id integer NOT NULL,
    searched_ids integer,
    articles integer,
    ids_in_use integer,
    urs integer,
    expert_db integer
);


CREATE TABLE rnacen.litsumm_summaries (
    id integer NOT NULL,
    rna_id text,
    context text,
    summary text,
    cost double precision,
    total_tokens integer,
    attempts integer,
    truthful boolean,
    problem_summary boolean,
    consistency_check_result text,
    selection_method text,
    rescue_prompts text[],
    primary_id character varying(44),
    display_id character varying(100),
    should_show boolean
);


CREATE TABLE rnacen.load_compara (
    homology_group text NOT NULL,
    ensembl_transcript text NOT NULL
);


CREATE TABLE rnacen.load_ensembl_pseudogenes (
    gene text NOT NULL,
    region_name text NOT NULL,
    chromosome text NOT NULL,
    strand integer NOT NULL,
    exon_start integer NOT NULL,
    exon_stop integer NOT NULL,
    assembly_id character varying(255) NOT NULL,
    exon_count integer NOT NULL
);


CREATE TABLE rnacen.load_genome_mapping_attempted (
    urs_taxid text NOT NULL,
    assembly_id text NOT NULL,
    last_run timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


CREATE TABLE rnacen.load_karyotypes (
    assembly_id character varying(255) NOT NULL,
    karyotype text
);


CREATE TABLE rnacen.load_model_assignments (
    model_name text NOT NULL,
    so_term_id text NOT NULL
);


CREATE TABLE rnacen.load_qa_rfam_attempted (
    urs text NOT NULL,
    model_source text NOT NULL,
    source_version text NOT NULL,
    last_run timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


CREATE TABLE rnacen.load_ref_pubmed (
    ref_pubmed_id integer,
    authors text,
    location text,
    title text,
    doi text
);


CREATE TABLE rnacen.load_rfam_go_terms (
    ontology_term_id character varying(10) NOT NULL,
    rfam_model_id character varying(20) NOT NULL
);


CREATE TABLE rnacen.load_rnc_coordinates (
    accession character varying(200),
    local_start bigint,
    local_end bigint,
    chromosome character varying(100),
    strand bigint,
    assembly_id character varying(255)
);


CREATE TABLE rnacen.load_rnc_secondary_structure (
    rnc_accession_id character varying(100),
    secondary_structure text,
    md5 character varying(32)
);


CREATE TABLE rnacen.load_rnc_text_mining (
    pattern_group text,
    pattern text,
    matching_word text,
    sentence text,
    md5 text,
    authors text,
    location text,
    title text,
    pmid text,
    doi text
);


CREATE TABLE rnacen.load_secondary_layout (
    urs text NOT NULL,
    secondary_structure text NOT NULL,
    model text NOT NULL,
    overlap_count integer NOT NULL,
    basepair_count integer NOT NULL,
    model_start integer,
    model_stop integer,
    model_coverage double precision,
    sequence_start integer,
    sequence_stop integer,
    sequence_coverage double precision
);


CREATE TABLE rnacen.r2dt_results (
    urs text NOT NULL,
    secondary_structure text NOT NULL,
    overlap_count integer NOT NULL,
    basepair_count integer NOT NULL,
    model_start integer,
    model_stop integer,
    sequence_start integer,
    sequence_stop integer,
    sequence_coverage double precision,
    model_id integer NOT NULL,
    id bigint NOT NULL,
    inferred_should_show boolean DEFAULT true NOT NULL,
    model_coverage double precision,
    assigned_should_show boolean
);


CREATE TABLE rnacen.load_secondary_layout_models (
    model_name text NOT NULL,
    taxid integer NOT NULL,
    rna_type text NOT NULL,
    so_term text NOT NULL,
    cell_location text NOT NULL,
    model_source text NOT NULL,
    model_length integer NOT NULL
);


CREATE TABLE rnacen.load_traveler_attempted (
    urs text NOT NULL,
    r2dt_version text
);


CREATE TABLE rnacen.old_summaries (
    id integer,
    rna_id text,
    context text,
    summary text,
    cost double precision,
    total_tokens integer,
    attempts integer,
    truthful boolean,
    problem_summary boolean,
    consistency_check_result text,
    selection_method text
);


CREATE TABLE rnacen.ontology_terms (
    ontology_term_id character varying(15) NOT NULL,
    ontology character varying(5),
    name text,
    definition text
);


CREATE TABLE rnacen.pipeline_tracking_genome_mapping (
    id bigint NOT NULL,
    urs_taxid text NOT NULL,
    assembly_id text NOT NULL,
    last_run timestamp without time zone NOT NULL
);


CREATE TABLE rnacen.pipeline_tracking_qa_scan (
    id bigint NOT NULL,
    urs text NOT NULL,
    model_source text NOT NULL,
    source_version text NOT NULL,
    last_run timestamp without time zone NOT NULL
);


CREATE TABLE rnacen.pipeline_tracking_traveler (
    id bigint NOT NULL,
    urs text NOT NULL,
    last_run timestamp without time zone NOT NULL,
    r2dt_version text
);


CREATE TABLE rnacen.protein_info (
    protein_accession text NOT NULL,
    description text,
    label text,
    synonyms text[]
);


CREATE TABLE rnacen.publications (
    id integer NOT NULL,
    database character varying(40) NOT NULL,
    total_ids integer NOT NULL,
    results integer NOT NULL
);


CREATE TABLE rnacen.qa_status (
    rna_id character varying(50) NOT NULL,
    upi character varying(30) NOT NULL,
    taxid integer NOT NULL,
    has_issue boolean NOT NULL,
    incomplete_sequence boolean NOT NULL,
    possible_contamination boolean NOT NULL,
    missing_rfam_match boolean NOT NULL,
    messages jsonb NOT NULL,
    from_repetitive_region boolean NOT NULL,
    possible_orf boolean NOT NULL
);


CREATE TABLE rnacen.r2dt_models (
    id integer NOT NULL,
    model_name text NOT NULL,
    taxid integer NOT NULL,
    cellular_location text,
    rna_type text NOT NULL,
    so_term_id text NOT NULL,
    model_source text NOT NULL,
    model_length integer NOT NULL,
    model_basepair_count integer NOT NULL
);


CREATE TABLE rnacen.release_stats (
    dbid bigint,
    this_release bigint NOT NULL,
    prev_release bigint,
    start_time timestamp without time zone,
    end_time timestamp without time zone,
    ff_loaded_rows bigint,
    retired_prev_releases bigint,
    retired_this_release bigint,
    retired_next_releases bigint,
    retired_total bigint,
    created_w_predecessors_v_1 bigint,
    created_w_predecessors_v_gt1 bigint,
    created_w_predecessors bigint,
    created_wo_predecessors_v_1 bigint,
    created_wo_predecessors_v_gt1 bigint,
    created_wo_predecessors bigint,
    active_created_prev_releases bigint,
    active_created_this_release bigint,
    active_created_next_releases bigint,
    created_this_release bigint,
    active_updated_this_release bigint,
    active_untouched_this_release bigint,
    active_total bigint,
    ff_taxid_nulls bigint
);


CREATE TABLE rnacen.rfam_analyzed_sequences (
    upi character varying(13) NOT NULL,
    date date NOT NULL,
    total_matches integer DEFAULT 0 NOT NULL,
    rfam_version character varying(5),
    total_family_matches integer DEFAULT 0 NOT NULL,
    CONSTRAINT rfam_analyzed_sequences_check CHECK ((total_matches >= total_family_matches)),
    CONSTRAINT rfam_analyzed_sequences_total_family_matches_check CHECK ((total_family_matches >= 0)),
    CONSTRAINT rfam_analyzed_sequences_total_matches_check CHECK ((total_matches >= 0))
);


CREATE TABLE rnacen.rfam_clans (
    rfam_clan_id character varying(20) NOT NULL,
    name text NOT NULL,
    description text NOT NULL,
    family_count integer NOT NULL,
    CONSTRAINT rfam_clans_family_count_check CHECK ((family_count >= 0))
);


CREATE TABLE rnacen.rfam_go_terms (
    rfam_go_term_id integer NOT NULL,
    rfam_model_id character varying(20) NOT NULL,
    ontology_term_id character varying(15) NOT NULL
);


CREATE TABLE rnacen.rfam_model_hits (
    rfam_hit_id integer NOT NULL,
    sequence_start integer NOT NULL,
    sequence_stop integer NOT NULL,
    sequence_completeness double precision NOT NULL,
    model_start integer NOT NULL,
    model_stop integer NOT NULL,
    model_completeness double precision NOT NULL,
    overlap character varying(30) NOT NULL,
    e_value double precision NOT NULL,
    score double precision NOT NULL,
    rfam_model_id character varying(20) NOT NULL,
    upi character varying(13) NOT NULL,
    rnc_sequence_features_id integer,
    CONSTRAINT rfam_model_hits_model_start_check CHECK ((model_start >= 0)),
    CONSTRAINT rfam_model_hits_model_stop_check CHECK ((model_stop >= 0)),
    CONSTRAINT rfam_model_hits_sequence_start_check CHECK ((sequence_start >= 0)),
    CONSTRAINT rfam_model_hits_sequence_stop_check CHECK ((sequence_stop >= 0))
);


CREATE TABLE rnacen.rfam_models (
    rfam_model_id character varying(20) NOT NULL,
    short_name character varying(50) NOT NULL,
    long_name character varying(200) NOT NULL,
    description character varying(2000),
    seed_count integer NOT NULL,
    full_count integer NOT NULL,
    length integer NOT NULL,
    is_suppressed boolean NOT NULL,
    domain character varying(50),
    rna_type character varying(250) NOT NULL,
    rfam_clan_id character varying(20),
    rfam_rna_type text NOT NULL,
    so_rna_type text,
    CONSTRAINT rfam_models_full_count_check CHECK ((full_count >= 0)),
    CONSTRAINT rfam_models_length_check CHECK ((length >= 0)),
    CONSTRAINT rfam_models_seed_count_check CHECK ((seed_count >= 0))
);


CREATE TABLE rnacen.rna (
    id bigint,
    upi character varying(30) NOT NULL,
    "timestamp" timestamp without time zone,
    userstamp character varying(60),
    crc64 character(16),
    len integer,
    seq_short character varying(4000),
    seq_long text,
    md5 character varying(64),
    CONSTRAINT constraint_rna_seq_min_length CHECK ((char_length((seq_short)::text) >= 10))
);


CREATE TABLE rnacen.rnc_accession_sequence_feature (
    id bigint NOT NULL,
    rnc_sequence_feature_id integer NOT NULL,
    accession text NOT NULL
);


CREATE TABLE rnacen.rnc_accession_sequence_region (
    id bigint NOT NULL,
    accession text NOT NULL,
    region_id bigint NOT NULL
);


CREATE TABLE rnacen.rnc_accessions (
    id bigint DEFAULT nextval('rnacen.rnc_accessions_seq'::regclass),
    accession character varying(200) NOT NULL,
    parent_ac character varying(200),
    seq_version bigint,
    feature_start bigint,
    feature_end bigint,
    feature_name character varying(80),
    ordinal bigint,
    division character varying(6),
    keywords character varying(200),
    description character varying(500),
    species character varying(300),
    organelle character varying(200),
    classification character varying(1000),
    project character varying(100),
    is_composite character varying(2),
    non_coding_id character varying(200),
    database character varying(40),
    external_id character varying(300),
    optional_id character varying(200),
    common_name character varying(200),
    allele character varying(100),
    anticodon character varying(200),
    chromosome character varying(200),
    experiment text,
    function character varying(4000),
    gene character varying(200),
    gene_synonym character varying(800),
    inference character varying(600),
    locus_tag character varying(100),
    map character varying(400),
    mol_type character varying(100),
    ncrna_class character varying(100),
    note text,
    old_locus_tag character varying(200),
    operon character varying(100),
    product character varying(600),
    pseudogene character varying(100),
    standard_name character varying(200),
    db_xref text,
    rna_type character varying(15),
    url text
)
WITH (autovacuum_vacuum_scale_factor='0.05');


CREATE TABLE rnacen.rnc_chemical_components (
    id character varying(16) NOT NULL,
    description character varying(1000),
    one_letter_code character varying(2),
    ccd_id character varying(6) DEFAULT 'NULL'::character varying,
    source character varying(20) DEFAULT 'NULL'::character varying,
    modomics_short_name character varying(40) DEFAULT 'NULL'::character varying
);


CREATE TABLE rnacen.rnc_database (
    id bigint NOT NULL,
    "timestamp" timestamp without time zone NOT NULL,
    userstamp character varying(30) NOT NULL,
    descr character varying(60) NOT NULL,
    current_release integer,
    full_descr character varying(1024),
    alive character varying(1) NOT NULL,
    for_release character(1),
    display_name character varying(60),
    project_id character varying(20),
    avg_length bigint,
    min_length bigint,
    max_length bigint,
    num_sequences bigint,
    num_organisms bigint,
    description character varying(1024),
    url character varying(500),
    example jsonb,
    reference jsonb
);


CREATE TABLE rnacen.rnc_database_json_stats (
    database character varying(40) NOT NULL,
    length_counts text,
    taxonomic_lineage text
);


CREATE TABLE rnacen.rnc_database_references (
    id bigint NOT NULL,
    dbid integer NOT NULL,
    reference_id bigint NOT NULL
);


CREATE TABLE rnacen.rnc_feedback_overlap (
    upi_taxid text NOT NULL,
    overlaps_with text[] NOT NULL,
    no_overlaps_with text[] NOT NULL,
    overlapping_upis text[],
    assembly_id character varying(255) NOT NULL,
    should_ignore boolean DEFAULT false,
    id bigint NOT NULL
);


CREATE TABLE rnacen.rnc_feedback_target_assemblies (
    assembly_id character varying(255),
    chromosome text,
    dbid bigint,
    database character varying(60)
);


CREATE TABLE rnacen.rnc_gene_status (
    id bigint NOT NULL,
    assembly_id text NOT NULL,
    urs_taxid text NOT NULL,
    region_id integer NOT NULL,
    status text NOT NULL
);


CREATE TABLE rnacen.rnc_import_tracker (
    id bigint NOT NULL,
    db_name character varying(60) NOT NULL,
    db_id bigint NOT NULL,
    last_import_date timestamp without time zone,
    file_md5 character varying(64)
);


CREATE TABLE rnacen.rnc_interactions (
    id bigint NOT NULL,
    intact_id text NOT NULL,
    urs_taxid text NOT NULL,
    interacting_id text NOT NULL,
    names jsonb NOT NULL,
    taxid integer NOT NULL
);


CREATE TABLE rnacen.rnc_locus (
    id bigint NOT NULL,
    assembly_id text NOT NULL,
    locus_name text NOT NULL,
    public_locus_name text NOT NULL,
    chromosome text NOT NULL,
    strand text NOT NULL,
    locus_start integer NOT NULL,
    locus_stop integer NOT NULL,
    member_count integer NOT NULL
);


CREATE TABLE rnacen.rnc_locus_members (
    id bigint NOT NULL,
    urs_taxid text NOT NULL,
    region_id integer NOT NULL,
    locus_id bigint NOT NULL,
    membership_status text NOT NULL
);


CREATE TABLE rnacen.rnc_modifications (
    id bigint NOT NULL,
    "position" bigint NOT NULL,
    author_assigned_position bigint NOT NULL,
    modification_id character varying(16) NOT NULL,
    upi character varying(26) NOT NULL,
    accession character varying(200) NOT NULL,
    CONSTRAINT sys_c0042332 CHECK (("position" >= 0))
);


CREATE TABLE rnacen.rnc_reference_map (
    id bigint DEFAULT nextval('rnacen.rnc_reference_map_seq'::regclass) NOT NULL,
    accession character varying(200) NOT NULL,
    reference_id bigint NOT NULL
)
WITH (autovacuum_vacuum_scale_factor='0.05');


CREATE TABLE rnacen.rnc_references (
    id bigint DEFAULT nextval('rnacen.rnc_refs_pk_seq'::regclass) NOT NULL,
    md5 character varying(64) NOT NULL,
    authors text,
    location character varying(4000),
    title character varying(4000),
    pmid character varying(40),
    pmcid character varying(40),
    epmcid character varying(40),
    doi character varying(160),
    epmc_updated smallint
);


CREATE TABLE rnacen.rnc_related_sequences (
    id integer NOT NULL,
    source_urs_taxid character varying(50) NOT NULL,
    source_accession character varying(100) NOT NULL,
    target_urs_taxid character varying(50),
    target_accession character varying(100) NOT NULL,
    methods text[],
    relationship_type text DEFAULT ''::text NOT NULL
);


CREATE TABLE rnacen.rnc_relationship_types (
    relationship_type text NOT NULL
);


CREATE TABLE rnacen.rnc_release (
    id bigint NOT NULL,
    dbid bigint NOT NULL,
    release_date timestamp without time zone NOT NULL,
    release_type character(1) NOT NULL,
    status character(1) NOT NULL,
    "timestamp" timestamp without time zone NOT NULL,
    userstamp character varying(30) NOT NULL,
    descr character varying(32),
    force_load character(1)
);


CREATE TABLE rnacen.rnc_rna_precomputed (
    id character varying(44) NOT NULL,
    taxid bigint,
    description character varying(500),
    upi character varying(26) NOT NULL,
    rna_type character varying(500) DEFAULT 'NULL'::character varying,
    update_date date DEFAULT ('now'::text)::date,
    has_coordinates boolean DEFAULT false,
    databases text,
    is_active boolean,
    last_release integer,
    short_description text,
    so_rna_type text,
    is_locus_representative boolean DEFAULT true,
    assigned_so_rna_type text
)
WITH (autovacuum_vacuum_scale_factor='0.05');


CREATE TABLE rnacen.rnc_secondary_structure (
    id integer,
    secondary_structure text,
    md5 character varying(32),
    rnc_accession_id character varying(100)
);


CREATE TABLE rnacen.rnc_sequence_exons (
    id integer NOT NULL,
    region_id integer,
    exon_start integer NOT NULL,
    exon_stop integer NOT NULL
);


CREATE TABLE rnacen.rnc_sequence_feature_providers (
    id bigint NOT NULL,
    name text,
    type text,
    description text
);


CREATE TABLE rnacen.rnc_sequence_feature_types (
    feature_name text NOT NULL,
    pretty_name text NOT NULL
);


CREATE TABLE rnacen.rnc_sequence_features (
    rnc_sequence_features_id integer NOT NULL,
    upi character varying(100) NOT NULL,
    taxid integer,
    accession character varying(100),
    start integer NOT NULL,
    stop integer NOT NULL,
    feature_name character varying(50) NOT NULL,
    metadata jsonb,
    feature_provider text
);


CREATE TABLE rnacen.rnc_sequence_regions (
    id integer NOT NULL,
    urs_taxid text NOT NULL,
    region_name text NOT NULL,
    chromosome text NOT NULL,
    strand integer NOT NULL,
    region_start integer NOT NULL,
    region_stop integer NOT NULL,
    assembly_id character varying(255),
    was_mapped boolean NOT NULL,
    identity double precision,
    providing_databases text[],
    exon_count integer NOT NULL,
    CONSTRAINT ck_rnc_sequence_regions__exon_count_size CHECK ((exon_count > 0))
);


CREATE TABLE rnacen.xref (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar])))
);


CREATE TABLE rnacen.rnc_taxonomy (
    id integer NOT NULL,
    name text NOT NULL,
    lineage text NOT NULL,
    aliases text[],
    replaced_by integer,
    common_name text,
    is_deleted boolean DEFAULT false NOT NULL,
    CONSTRAINT rnc_taxonomy_common_name_check CHECK ((common_name <> ''::text))
);


CREATE TABLE rnacen.temp_bad_is_active (
    urs_taxid text,
    is_active boolean,
    all_deleted boolean
);


CREATE TABLE rnacen.validate_layout_counts (
    name text NOT NULL,
    changed integer NOT NULL,
    unchanged integer NOT NULL,
    inserted integer NOT NULL,
    moved integer NOT NULL,
    rotated integer NOT NULL,
    total integer NOT NULL,
    rna_length integer,
    not_drawn integer,
    overlap_count integer
);


CREATE TABLE rnacen.validate_layout_hits (
    urs text,
    sequence_rna_type character varying(500),
    sequence_taxid bigint,
    model_name text,
    model_rna_type text,
    model_source text,
    model_so_rna_type text,
    model_taxid integer
);


CREATE TABLE rnacen.xref_not_unique (
    dbid smallint NOT NULL,
    release_id integer NOT NULL,
    upi character(13) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(30) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(100) NOT NULL,
    version integer,
    taxid bigint
);


CREATE TABLE rnacen.xref_p10_deleted (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p10_deleted_check CHECK (((dbid = 10) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p10_not_deleted (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p10_not_deleted_check CHECK (((dbid = 10) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p11_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p11_deleted_check CHECK (((dbid = 11) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p11_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p11_deleted_check CHECK (((dbid = 11) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p11_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p11_not_deleted_check CHECK (((dbid = 11) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p11_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p11_not_deleted_check CHECK (((dbid = 11) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p12_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p12_deleted_check CHECK (((dbid = 12) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p12_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p12_deleted_check CHECK (((dbid = 12) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p12_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p12_not_deleted_check CHECK (((dbid = 12) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p12_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p12_not_deleted_check CHECK (((dbid = 12) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p13_deleted (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p13_deleted_check CHECK (((dbid = 13) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p13_not_deleted (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p13_not_deleted_check CHECK (((dbid = 13) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p14_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p14_deleted_check CHECK (((dbid = 14) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p14_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p14_deleted_check CHECK (((dbid = 14) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p14_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p14_not_deleted_check CHECK (((dbid = 14) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p14_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p14_not_deleted_check CHECK (((dbid = 14) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p15_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p15_deleted_check CHECK (((dbid = 15) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p15_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p15_deleted_check CHECK (((dbid = 15) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p15_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p15_not_deleted_check CHECK (((dbid = 15) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p15_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p15_not_deleted_check CHECK (((dbid = 15) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p16_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p16_deleted_check CHECK (((dbid = 16) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p16_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p16_deleted_check CHECK (((dbid = 16) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p16_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p16_not_deleted_check CHECK (((dbid = 16) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p16_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p16_not_deleted_check CHECK (((dbid = 16) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p17_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p17_deleted_check CHECK (((dbid = 17) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p17_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p17_deleted_check CHECK (((dbid = 17) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p17_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p17_not_deleted_check CHECK (((dbid = 17) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p17_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p17_not_deleted_check CHECK (((dbid = 17) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p18_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p18_deleted_check CHECK (((dbid = 18) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p18_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p18_deleted_check CHECK (((dbid = 18) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p18_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p18_not_deleted_check CHECK (((dbid = 18) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p18_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p18_not_deleted_check CHECK (((dbid = 18) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p19_deleted (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p19_deleted_check CHECK (((dbid = 19) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p19_not_deleted (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p19_not_deleted_check CHECK (((dbid = 19) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p1_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p1_deleted_check CHECK (((dbid = 1) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p1_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p1_deleted_check CHECK (((dbid = 1) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p1_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p1_not_deleted_check CHECK (((dbid = 1) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p1_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p1_not_deleted_check CHECK (((dbid = 1) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p20_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p20_deleted_check CHECK (((dbid = 20) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p20_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p20_deleted_check CHECK (((dbid = 20) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p20_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p20_not_deleted_check CHECK (((dbid = 20) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p20_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p20_not_deleted_check CHECK (((dbid = 20) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p21_deleted (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p21_deleted_check CHECK (((dbid = 21) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p21_not_deleted (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p21_not_deleted_check CHECK (((dbid = 21) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p22_deleted (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p22_deleted_check CHECK (((dbid = 22) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p22_not_deleted (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p22_not_deleted_check CHECK (((dbid = 22) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p23_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p23_deleted_check CHECK (((dbid = 23) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p23_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p23_deleted_check CHECK (((dbid = 23) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p23_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p23_not_deleted_check CHECK (((dbid = 23) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p23_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p23_not_deleted_check CHECK (((dbid = 23) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p24_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p24_deleted_check CHECK (((dbid = 24) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p24_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p24_deleted_check CHECK (((dbid = 24) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p24_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p24_not_deleted_check CHECK (((dbid = 24) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p24_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p24_not_deleted_check CHECK (((dbid = 24) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p25_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p25_deleted_check CHECK (((dbid = 25) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p25_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p25_deleted_check CHECK (((dbid = 25) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p25_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p25_not_deleted_check CHECK (((dbid = 25) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p25_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p25_not_deleted_check CHECK (((dbid = 25) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p26_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p26_deleted_check CHECK (((dbid = 26) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p26_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p26_deleted_check CHECK (((dbid = 26) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p26_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p26_not_deleted_check CHECK (((dbid = 26) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p26_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p26_not_deleted_check CHECK (((dbid = 26) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p27_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p27_deleted_check CHECK (((dbid = 27) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p27_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p27_not_deleted_check CHECK (((dbid = 27) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p28_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p28_deleted_check CHECK (((dbid = 28) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p28_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p28_not_deleted_check CHECK (((dbid = 28) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p29_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p29_deleted_check CHECK (((dbid = 29) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p29_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p29_not_deleted_check CHECK (((dbid = 29) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p2_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p2_deleted_check CHECK (((dbid = 2) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p2_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p2_deleted_check CHECK (((dbid = 2) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p2_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p2_not_deleted_check CHECK (((dbid = 2) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p2_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p2_not_deleted_check CHECK (((dbid = 2) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p30_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p30_deleted_check CHECK (((dbid = 30) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p30_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p30_deleted_check CHECK (((dbid = 30) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p30_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p30_not_deleted_check CHECK (((dbid = 30) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p30_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p30_not_deleted_check CHECK (((dbid = 30) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p31_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p31_deleted_check CHECK (((dbid = 31) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p31_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p31_deleted_check CHECK (((dbid = 31) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p31_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p31_not_deleted_check CHECK (((dbid = 31) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p31_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p31_not_deleted_check CHECK (((dbid = 31) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p32_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p32_deleted_check CHECK (((dbid = 32) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p32_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p32_not_deleted_check CHECK (((dbid = 32) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p33_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p33_deleted_check CHECK (((dbid = 33) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p33_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p33_deleted_check CHECK (((dbid = 33) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p33_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p33_not_deleted_check CHECK (((dbid = 33) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p33_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p33_not_deleted_check CHECK (((dbid = 33) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p34_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p34_deleted_check CHECK (((dbid = 34) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p34_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p34_deleted_check CHECK (((dbid = 34) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p34_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p34_not_deleted_check CHECK (((dbid = 34) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p34_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p34_not_deleted_check CHECK (((dbid = 34) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p35_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p35_deleted_check CHECK (((dbid = 35) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p35_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p35_deleted_check CHECK (((dbid = 35) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p35_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p35_not_deleted_check CHECK (((dbid = 35) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p35_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p35_not_deleted_check CHECK (((dbid = 35) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p36_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p36_deleted_check CHECK (((dbid = 36) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p36_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p36_deleted_check CHECK (((dbid = 36) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p36_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p36_not_deleted_check CHECK (((dbid = 36) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p36_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p36_not_deleted_check CHECK (((dbid = 36) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p37_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p37_deleted_check CHECK (((dbid = 37) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p37_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p37_deleted_check CHECK (((dbid = 37) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p37_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p37_not_deleted_check CHECK (((dbid = 37) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p37_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p37_not_deleted_check CHECK (((dbid = 37) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p38_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p38_deleted_check CHECK (((dbid = 38) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p38_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p38_deleted_check CHECK (((dbid = 38) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p38_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p38_not_deleted_check CHECK (((dbid = 38) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p38_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p38_not_deleted_check CHECK (((dbid = 38) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p39_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p39_deleted_check CHECK (((dbid = 39) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p39_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p39_deleted_check CHECK (((dbid = 39) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p39_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p39_not_deleted_check CHECK (((dbid = 39) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p39_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p39_not_deleted_check CHECK (((dbid = 39) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p3_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p3_deleted_check CHECK (((dbid = 3) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p3_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p3_deleted_check CHECK (((dbid = 3) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p3_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p3_not_deleted_check CHECK (((dbid = 3) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p3_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p3_not_deleted_check CHECK (((dbid = 3) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p40_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p40_deleted_check CHECK (((dbid = 40) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p40_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p40_deleted_check CHECK (((dbid = 40) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p40_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p40_not_deleted_check CHECK (((dbid = 40) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p40_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p40_not_deleted_check CHECK (((dbid = 40) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p41_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p41_deleted_check CHECK (((dbid = 41) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p41_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p41_deleted_check CHECK (((dbid = 41) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p41_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p41_not_deleted_check CHECK (((dbid = 41) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p41_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p41_not_deleted_check CHECK (((dbid = 41) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p42_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p42_deleted_check CHECK (((dbid = 42) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p42_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p42_deleted_check CHECK (((dbid = 42) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p42_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p42_not_deleted_check CHECK (((dbid = 42) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p42_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p42_not_deleted_check CHECK (((dbid = 42) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p43_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p43_deleted_check CHECK (((dbid = 43) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p43_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint NOT NULL,
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p43_deleted_check CHECK (((dbid = 43) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p43_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p43_not_deleted_check CHECK (((dbid = 43) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p43_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint NOT NULL,
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p43_not_deleted_check CHECK (((dbid = 43) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p44_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p44_deleted_check CHECK (((dbid = 44) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p44_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p44_deleted_check CHECK (((dbid = 44) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p44_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p44_not_deleted_check CHECK (((dbid = 44) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p44_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p44_not_deleted_check CHECK (((dbid = 44) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p45_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p45_deleted_check CHECK (((dbid = 45) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p45_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p45_deleted_check CHECK (((dbid = 45) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p45_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p45_not_deleted_check CHECK (((dbid = 45) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p45_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p45_not_deleted_check CHECK (((dbid = 45) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p46_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p46_deleted_check CHECK (((dbid = 46) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p46_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p46_deleted_check CHECK (((dbid = 46) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p46_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p46_not_deleted_check CHECK (((dbid = 46) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p46_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p46_not_deleted_check CHECK (((dbid = 46) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p47_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p47_deleted_check CHECK (((dbid = 47) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p47_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p47_deleted_check CHECK (((dbid = 47) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p47_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p47_not_deleted_check CHECK (((dbid = 47) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p47_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p47_not_deleted_check CHECK (((dbid = 47) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p48_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p48_deleted_check CHECK (((dbid = 48) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p48_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p48_deleted_check CHECK (((dbid = 48) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p48_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p48_not_deleted_check CHECK (((dbid = 48) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p48_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p48_not_deleted_check CHECK (((dbid = 48) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p49_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p49_deleted_check CHECK (((dbid = 49) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p49_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint NOT NULL,
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p49_deleted_check CHECK (((dbid = 49) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p49_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p49_not_deleted_check CHECK (((dbid = 49) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p49_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint NOT NULL,
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p49_not_deleted_check CHECK (((dbid = 49) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p4_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p4_deleted_check CHECK (((dbid = 4) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p4_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p4_deleted_check CHECK (((dbid = 4) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p4_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p4_not_deleted_check CHECK (((dbid = 4) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p4_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p4_not_deleted_check CHECK (((dbid = 4) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p50_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p50_deleted_check CHECK (((dbid = 50) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p50_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p50_deleted_check CHECK (((dbid = 50) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p50_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p50_not_deleted_check CHECK (((dbid = 50) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p50_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p50_not_deleted_check CHECK (((dbid = 50) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p51_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p51_deleted_check CHECK (((dbid = 51) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p51_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint NOT NULL,
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p51_deleted_check CHECK (((dbid = 51) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p51_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p51_not_deleted_check CHECK (((dbid = 51) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p51_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint NOT NULL,
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p51_not_deleted_check CHECK (((dbid = 51) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p52_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p52_deleted_check CHECK (((dbid = 52) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p52_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p52_deleted_check CHECK (((dbid = 52) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p52_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p52_not_deleted_check CHECK (((dbid = 52) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p52_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p52_not_deleted_check CHECK (((dbid = 52) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p53_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p53_deleted_check CHECK (((dbid = 53) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p53_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint NOT NULL,
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p53_deleted_check CHECK (((dbid = 53) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p53_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p53_not_deleted_check CHECK (((dbid = 53) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p53_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint NOT NULL,
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p53_not_deleted_check CHECK (((dbid = 53) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p54_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint NOT NULL,
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p54_deleted_check CHECK (((dbid = 54) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p54_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint NOT NULL,
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p54_not_deleted_check CHECK (((dbid = 54) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p55_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p55_deleted_check CHECK (((dbid = 55) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p55_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p55_deleted_check CHECK (((dbid = 55) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p55_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p55_not_deleted_check CHECK (((dbid = 55) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p55_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p55_not_deleted_check CHECK (((dbid = 55) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p5_deleted (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p5_deleted_check CHECK (((dbid = 5) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p5_not_deleted (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p5_not_deleted_check CHECK (((dbid = 5) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p6_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p6_deleted_check CHECK (((dbid = 6) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p6_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p6_deleted_check CHECK (((dbid = 6) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p6_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p6_not_deleted_check CHECK (((dbid = 6) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p6_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p6_not_deleted_check CHECK (((dbid = 6) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p7_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p7_deleted_check CHECK (((dbid = 7) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p7_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p7_deleted_check CHECK (((dbid = 7) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p7_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p7_not_deleted_check CHECK (((dbid = 7) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p7_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p7_not_deleted_check CHECK (((dbid = 7) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p8_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p8_deleted_check CHECK (((dbid = 8) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p8_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p8_deleted_check CHECK (((dbid = 8) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p8_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p8_not_deleted_check CHECK (((dbid = 8) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p8_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p8_not_deleted_check CHECK (((dbid = 8) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p9_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p9_deleted_check CHECK (((dbid = 9) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p9_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p9_deleted_check CHECK (((dbid = 9) AND (deleted = 'Y'::bpchar)))
);


CREATE TABLE rnacen.xref_p9_not_deleted (
    dbid smallint,
    created integer,
    last integer,
    upi character varying(26),
    version_i integer,
    deleted character(1),
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone,
    userstamp character varying(20) DEFAULT 'USER'::character varying,
    ac character varying(300),
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p9_not_deleted_check CHECK (((dbid = 9) AND (deleted = 'N'::bpchar)))
);


CREATE TABLE rnacen.xref_p9_not_deleted_old (
    dbid smallint NOT NULL,
    created integer NOT NULL,
    last integer NOT NULL,
    upi character varying(26) NOT NULL,
    version_i integer NOT NULL,
    deleted character(1) NOT NULL,
    "timestamp" timestamp without time zone DEFAULT ('now'::text)::timestamp without time zone NOT NULL,
    userstamp character varying(20) DEFAULT 'USER'::character varying NOT NULL,
    ac character varying(300) NOT NULL,
    version integer,
    taxid bigint,
    id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass),
    CONSTRAINT "ck_xref$deleted" CHECK ((deleted = ANY (ARRAY['Y'::bpchar, 'N'::bpchar]))),
    CONSTRAINT xref_p9_not_deleted_check CHECK (((dbid = 9) AND (deleted = 'N'::bpchar)))
);


ALTER TABLE ONLY rnacen.auth_group ALTER COLUMN id SET DEFAULT nextval('rnacen.auth_group_id_seq'::regclass);


ALTER TABLE ONLY rnacen.auth_group_permissions ALTER COLUMN id SET DEFAULT nextval('rnacen.auth_group_permissions_id_seq'::regclass);


ALTER TABLE ONLY rnacen.auth_permission ALTER COLUMN id SET DEFAULT nextval('rnacen.auth_permission_id_seq'::regclass);


ALTER TABLE ONLY rnacen.auth_user ALTER COLUMN id SET DEFAULT nextval('rnacen.auth_user_id_seq'::regclass);


ALTER TABLE ONLY rnacen.auth_user_groups ALTER COLUMN id SET DEFAULT nextval('rnacen.auth_user_groups_id_seq'::regclass);


ALTER TABLE ONLY rnacen.auth_user_user_permissions ALTER COLUMN id SET DEFAULT nextval('rnacen.auth_user_user_permissions_id_seq'::regclass);


ALTER TABLE ONLY rnacen.bad_precompute ALTER COLUMN id SET DEFAULT nextval('rnacen.bad_precompute_id_seq'::regclass);


ALTER TABLE ONLY rnacen.blog ALTER COLUMN id SET DEFAULT nextval('rnacen.blog_id_seq'::regclass);


ALTER TABLE ONLY rnacen.corsheaders_corsmodel ALTER COLUMN id SET DEFAULT nextval('rnacen.corsheaders_corsmodel_id_seq'::regclass);


ALTER TABLE ONLY rnacen.django_admin_log ALTER COLUMN id SET DEFAULT nextval('rnacen.django_admin_log_id_seq'::regclass);


ALTER TABLE ONLY rnacen.django_content_type ALTER COLUMN id SET DEFAULT nextval('rnacen.django_content_type_id_seq'::regclass);


ALTER TABLE ONLY rnacen.django_migrations ALTER COLUMN id SET DEFAULT nextval('rnacen.django_migrations_id_seq'::regclass);


ALTER TABLE ONLY rnacen.django_site ALTER COLUMN id SET DEFAULT nextval('rnacen.django_site_id_seq'::regclass);


ALTER TABLE ONLY rnacen.ensembl_compara ALTER COLUMN id SET DEFAULT nextval('rnacen.ensembl_compara_id_seq'::regclass);


ALTER TABLE ONLY rnacen.ensembl_coordinate_systems ALTER COLUMN id SET DEFAULT nextval('rnacen.ensembl_coordinate_systems_id_seq'::regclass);


ALTER TABLE ONLY rnacen.ensembl_import_tracking ALTER COLUMN id SET DEFAULT nextval('rnacen.ensembl_import_tracking_id_seq'::regclass);


ALTER TABLE ONLY rnacen.ensembl_karyotype ALTER COLUMN id SET DEFAULT nextval('rnacen.ensembl_karyotype_id_seq'::regclass);


ALTER TABLE ONLY rnacen.ensembl_pseudogene_exons ALTER COLUMN id SET DEFAULT nextval('rnacen.ensembl_pseudogene_exons_id_seq'::regclass);


ALTER TABLE ONLY rnacen.ensembl_pseudogene_regions ALTER COLUMN id SET DEFAULT nextval('rnacen.ensembl_pseudogene_regions_id_seq'::regclass);


ALTER TABLE ONLY rnacen.go_term_annotations ALTER COLUMN go_term_annotation_id SET DEFAULT nextval('rnacen.go_term_annotations_go_term_annotation_id_seq'::regclass);


ALTER TABLE ONLY rnacen.go_term_publication_map ALTER COLUMN go_term_publication_mapping_id SET DEFAULT nextval('rnacen.go_term_publication_map_go_term_publication_mapping_id_seq1'::regclass);


ALTER TABLE ONLY rnacen.litscan_statistics ALTER COLUMN id SET DEFAULT nextval('rnacen.litscan_statistics_id_seq'::regclass);


ALTER TABLE ONLY rnacen.litsumm_summaries ALTER COLUMN id SET DEFAULT nextval('rnacen.litsumm_summaries_id_seq'::regclass);


ALTER TABLE ONLY rnacen.pipeline_tracking_genome_mapping ALTER COLUMN id SET DEFAULT nextval('rnacen.pipeline_tracking_genome_mapping_id_seq'::regclass);


ALTER TABLE ONLY rnacen.pipeline_tracking_qa_scan ALTER COLUMN id SET DEFAULT nextval('rnacen.pipeline_tracking_scan_id_seq'::regclass);


ALTER TABLE ONLY rnacen.pipeline_tracking_traveler ALTER COLUMN id SET DEFAULT nextval('rnacen.pipeline_tracking_traveler_id_seq'::regclass);


ALTER TABLE ONLY rnacen.publications ALTER COLUMN id SET DEFAULT nextval('rnacen.publications_id_seq'::regclass);


ALTER TABLE ONLY rnacen.r2dt_models ALTER COLUMN id SET DEFAULT nextval('rnacen.rnc_secondary_structure_layout_models_id_seq'::regclass);


ALTER TABLE ONLY rnacen.r2dt_results ALTER COLUMN id SET DEFAULT nextval('rnacen.load_secondary_layout_id_seq'::regclass);


ALTER TABLE ONLY rnacen.rfam_go_terms ALTER COLUMN rfam_go_term_id SET DEFAULT nextval('rnacen.rfam_go_terms_rfam_go_term_id_seq'::regclass);


ALTER TABLE ONLY rnacen.rfam_model_hits ALTER COLUMN rfam_hit_id SET DEFAULT nextval('rnacen.rfam_model_hits_rfam_hit_id_seq'::regclass);


ALTER TABLE ONLY rnacen.rnc_accession_sequence_feature ALTER COLUMN id SET DEFAULT nextval('rnacen.rnc_accessions_sequence_features_id_seq'::regclass);


ALTER TABLE ONLY rnacen.rnc_accession_sequence_region ALTER COLUMN id SET DEFAULT nextval('rnacen.rnc_accession_sequence_region_id_seq'::regclass);


ALTER TABLE ONLY rnacen.rnc_database_references ALTER COLUMN id SET DEFAULT nextval('rnacen.rnc_database_references_id_seq'::regclass);


ALTER TABLE ONLY rnacen.rnc_feedback_overlap ALTER COLUMN id SET DEFAULT nextval('rnacen.rnc_feedback_overlap_id_seq'::regclass);


ALTER TABLE ONLY rnacen.rnc_gene_status ALTER COLUMN id SET DEFAULT nextval('rnacen.rnc_gene_status_id_seq'::regclass);


ALTER TABLE ONLY rnacen.rnc_import_tracker ALTER COLUMN id SET DEFAULT nextval('rnacen.rnc_import_tracker_id_seq'::regclass);


ALTER TABLE ONLY rnacen.rnc_import_tracker ALTER COLUMN db_id SET DEFAULT nextval('rnacen.rnc_import_tracker_db_id_seq'::regclass);


ALTER TABLE ONLY rnacen.rnc_interactions ALTER COLUMN id SET DEFAULT nextval('rnacen.rnc_interactions_id_seq'::regclass);


ALTER TABLE ONLY rnacen.rnc_locus ALTER COLUMN id SET DEFAULT nextval('rnacen.rnc_locus_id_seq'::regclass);


ALTER TABLE ONLY rnacen.rnc_locus_members ALTER COLUMN id SET DEFAULT nextval('rnacen.rnc_locus_members_id_seq'::regclass);


ALTER TABLE ONLY rnacen.rnc_related_sequences ALTER COLUMN id SET DEFAULT nextval('rnacen.rnc_related_sequences_id_seq'::regclass);


ALTER TABLE ONLY rnacen.rnc_sequence_exons ALTER COLUMN id SET DEFAULT nextval('rnacen.rnc_sequence_exons_id_seq'::regclass);


ALTER TABLE ONLY rnacen.rnc_sequence_feature_providers ALTER COLUMN id SET DEFAULT nextval('rnacen.rnc_sequence_feature_providers_id_seq'::regclass);


ALTER TABLE ONLY rnacen.rnc_sequence_features ALTER COLUMN rnc_sequence_features_id SET DEFAULT nextval('rnacen.rnc_sequence_features_rnc_sequence_features_id_seq'::regclass);


ALTER TABLE ONLY rnacen.rnc_sequence_regions ALTER COLUMN id SET DEFAULT nextval('rnacen.rnc_sequence_regions_id_seq'::regclass);


ALTER TABLE ONLY rnacen.xref_p10_deleted ALTER COLUMN "timestamp" SET DEFAULT ('now'::text)::timestamp without time zone;


ALTER TABLE ONLY rnacen.xref_p10_deleted ALTER COLUMN userstamp SET DEFAULT 'USER'::character varying;


ALTER TABLE ONLY rnacen.xref_p10_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_pk_seq'::regclass);


ALTER TABLE ONLY rnacen.xref_p10_not_deleted ALTER COLUMN "timestamp" SET DEFAULT ('now'::text)::timestamp without time zone;


ALTER TABLE ONLY rnacen.xref_p10_not_deleted ALTER COLUMN userstamp SET DEFAULT 'USER'::character varying;


ALTER TABLE ONLY rnacen.xref_p10_not_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_pk_seq'::regclass);


ALTER TABLE ONLY rnacen.xref_p13_deleted ALTER COLUMN "timestamp" SET DEFAULT ('now'::text)::timestamp without time zone;


ALTER TABLE ONLY rnacen.xref_p13_deleted ALTER COLUMN userstamp SET DEFAULT 'USER'::character varying;


ALTER TABLE ONLY rnacen.xref_p13_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_pk_seq'::regclass);


ALTER TABLE ONLY rnacen.xref_p13_not_deleted ALTER COLUMN "timestamp" SET DEFAULT ('now'::text)::timestamp without time zone;


ALTER TABLE ONLY rnacen.xref_p13_not_deleted ALTER COLUMN userstamp SET DEFAULT 'USER'::character varying;


ALTER TABLE ONLY rnacen.xref_p13_not_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_pk_seq'::regclass);


ALTER TABLE ONLY rnacen.xref_p19_deleted ALTER COLUMN "timestamp" SET DEFAULT ('now'::text)::timestamp without time zone;


ALTER TABLE ONLY rnacen.xref_p19_deleted ALTER COLUMN userstamp SET DEFAULT 'USER'::character varying;


ALTER TABLE ONLY rnacen.xref_p19_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_pk_seq'::regclass);


ALTER TABLE ONLY rnacen.xref_p19_not_deleted ALTER COLUMN "timestamp" SET DEFAULT ('now'::text)::timestamp without time zone;


ALTER TABLE ONLY rnacen.xref_p19_not_deleted ALTER COLUMN userstamp SET DEFAULT 'USER'::character varying;


ALTER TABLE ONLY rnacen.xref_p19_not_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_pk_seq'::regclass);


ALTER TABLE ONLY rnacen.xref_p21_deleted ALTER COLUMN "timestamp" SET DEFAULT ('now'::text)::timestamp without time zone;


ALTER TABLE ONLY rnacen.xref_p21_deleted ALTER COLUMN userstamp SET DEFAULT 'USER'::character varying;


ALTER TABLE ONLY rnacen.xref_p21_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_pk_seq'::regclass);


ALTER TABLE ONLY rnacen.xref_p21_not_deleted ALTER COLUMN "timestamp" SET DEFAULT ('now'::text)::timestamp without time zone;


ALTER TABLE ONLY rnacen.xref_p21_not_deleted ALTER COLUMN userstamp SET DEFAULT 'USER'::character varying;


ALTER TABLE ONLY rnacen.xref_p21_not_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_pk_seq'::regclass);


ALTER TABLE ONLY rnacen.xref_p22_deleted ALTER COLUMN "timestamp" SET DEFAULT ('now'::text)::timestamp without time zone;


ALTER TABLE ONLY rnacen.xref_p22_deleted ALTER COLUMN userstamp SET DEFAULT 'USER'::character varying;


ALTER TABLE ONLY rnacen.xref_p22_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_pk_seq'::regclass);


ALTER TABLE ONLY rnacen.xref_p22_not_deleted ALTER COLUMN "timestamp" SET DEFAULT ('now'::text)::timestamp without time zone;


ALTER TABLE ONLY rnacen.xref_p22_not_deleted ALTER COLUMN userstamp SET DEFAULT 'USER'::character varying;


ALTER TABLE ONLY rnacen.xref_p22_not_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_pk_seq'::regclass);


ALTER TABLE ONLY rnacen.xref_p43_deleted_old ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_p43_deleted_id_seq'::regclass);


ALTER TABLE ONLY rnacen.xref_p43_not_deleted_old ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_p43_not_deleted_id_seq'::regclass);


ALTER TABLE ONLY rnacen.xref_p49_deleted_old ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_p49_deleted_id_seq'::regclass);


ALTER TABLE ONLY rnacen.xref_p49_not_deleted_old ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_p49_not_deleted_id_seq'::regclass);


ALTER TABLE ONLY rnacen.xref_p51_deleted_old ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_p51_deleted_id_seq'::regclass);


ALTER TABLE ONLY rnacen.xref_p51_not_deleted_old ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_p51_not_deleted_id_seq'::regclass);


ALTER TABLE ONLY rnacen.xref_p53_deleted_old ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_p53_deleted_id_seq'::regclass);


ALTER TABLE ONLY rnacen.xref_p53_not_deleted_old ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_p53_not_deleted_id_seq'::regclass);


ALTER TABLE ONLY rnacen.xref_p54_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_p54_deleted_id_seq'::regclass);


ALTER TABLE ONLY rnacen.xref_p54_not_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_p54_not_deleted_id_seq'::regclass);


ALTER TABLE ONLY rnacen.xref_p5_deleted ALTER COLUMN "timestamp" SET DEFAULT ('now'::text)::timestamp without time zone;


ALTER TABLE ONLY rnacen.xref_p5_deleted ALTER COLUMN userstamp SET DEFAULT 'USER'::character varying;


ALTER TABLE ONLY rnacen.xref_p5_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_pk_seq'::regclass);


ALTER TABLE ONLY rnacen.xref_p5_not_deleted ALTER COLUMN "timestamp" SET DEFAULT ('now'::text)::timestamp without time zone;


ALTER TABLE ONLY rnacen.xref_p5_not_deleted ALTER COLUMN userstamp SET DEFAULT 'USER'::character varying;


ALTER TABLE ONLY rnacen.xref_p5_not_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_pk_seq'::regclass);


ALTER TABLE ONLY rnacen.auth_group
    ADD CONSTRAINT auth_group_name_key UNIQUE (name);


ALTER TABLE ONLY rnacen.auth_group_permissions
    ADD CONSTRAINT auth_group_permissions_group_id_permission_id_key UNIQUE (group_id, permission_id);


ALTER TABLE ONLY rnacen.auth_group_permissions
    ADD CONSTRAINT auth_group_permissions_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.auth_group
    ADD CONSTRAINT auth_group_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.auth_permission
    ADD CONSTRAINT auth_permission_content_type_id_codename_key UNIQUE (content_type_id, codename);


ALTER TABLE ONLY rnacen.auth_permission
    ADD CONSTRAINT auth_permission_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.auth_user_groups
    ADD CONSTRAINT auth_user_groups_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.auth_user_groups
    ADD CONSTRAINT auth_user_groups_user_id_group_id_key UNIQUE (user_id, group_id);


ALTER TABLE ONLY rnacen.auth_user
    ADD CONSTRAINT auth_user_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.auth_user_user_permissions
    ADD CONSTRAINT auth_user_user_permissions_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.auth_user_user_permissions
    ADD CONSTRAINT auth_user_user_permissions_user_id_permission_id_key UNIQUE (user_id, permission_id);


ALTER TABLE ONLY rnacen.auth_user
    ADD CONSTRAINT auth_user_username_key UNIQUE (username);


ALTER TABLE ONLY rnacen.bad_precompute
    ADD CONSTRAINT bad_precompute_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.blog
    ADD CONSTRAINT blog_pkey PRIMARY KEY (id);


ALTER TABLE rnacen.rna
    ADD CONSTRAINT constraint_rna_seq_long_n CHECK (((seq_long IS NULL) OR (((len - length(replace(seq_long, 'N'::text, ''::text))))::numeric <= round((0.1 * (len)::numeric))))) NOT VALID;


ALTER TABLE rnacen.rna
    ADD CONSTRAINT constraint_rna_seq_short_n CHECK (((seq_short IS NULL) OR (((len - length(replace((seq_short)::text, 'N'::text, ''::text))))::numeric <= round((0.1 * (len)::numeric))))) NOT VALID;


ALTER TABLE ONLY rnacen.corsheaders_corsmodel
    ADD CONSTRAINT corsheaders_corsmodel_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.django_admin_log
    ADD CONSTRAINT django_admin_log_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.django_content_type
    ADD CONSTRAINT django_content_type_app_label_45f3b1d93ec8c61c_uniq UNIQUE (app_label, model);


ALTER TABLE ONLY rnacen.django_content_type
    ADD CONSTRAINT django_content_type_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.django_migrations
    ADD CONSTRAINT django_migrations_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.django_session
    ADD CONSTRAINT django_session_pkey PRIMARY KEY (session_key);


ALTER TABLE ONLY rnacen.django_site
    ADD CONSTRAINT django_site_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.ensembl_assembly
    ADD CONSTRAINT ensembl_assembly_pkey PRIMARY KEY (assembly_id);


ALTER TABLE ONLY rnacen.ensembl_coordinate_systems
    ADD CONSTRAINT ensembl_coordinate_systems_chromosome_assembly_id_key UNIQUE (chromosome, assembly_id);


ALTER TABLE ONLY rnacen.ensembl_coordinate_systems
    ADD CONSTRAINT ensembl_coordinate_systems_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.ensembl_import_tracking
    ADD CONSTRAINT ensembl_import_tracking_database_name_task_name_key UNIQUE (database_name, task_name);


ALTER TABLE ONLY rnacen.ensembl_import_tracking
    ADD CONSTRAINT ensembl_import_tracking_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.ensembl_karyotype
    ADD CONSTRAINT ensembl_karyotype_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.ensembl_pseudogene_exons
    ADD CONSTRAINT ensembl_pseudogene_exons_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.ensembl_pseudogene_regions
    ADD CONSTRAINT ensembl_pseudogene_regions_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.go_term_annotations
    ADD CONSTRAINT go_term_annotations_pkey PRIMARY KEY (go_term_annotation_id);


ALTER TABLE ONLY rnacen.go_term_publication_map
    ADD CONSTRAINT go_term_publication_map_pkey1 PRIMARY KEY (go_term_publication_mapping_id);


ALTER TABLE ONLY rnacen.insdc_so_term_mapping
    ADD CONSTRAINT insdc_so_term_mapping_pkey PRIMARY KEY (rna_type);


ALTER TABLE ONLY rnacen.litscan_statistics
    ADD CONSTRAINT litscan_statistics_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.litsumm_summaries
    ADD CONSTRAINT litsumm_summaries_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.r2dt_results
    ADD CONSTRAINT load_secondary_layout_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.load_traveler_attempted
    ADD CONSTRAINT load_traveler_attempted_pkey PRIMARY KEY (urs);


ALTER TABLE ONLY rnacen.rnc_sequence_feature_providers
    ADD CONSTRAINT name_unique UNIQUE (name);


ALTER TABLE ONLY rnacen.ontology_terms
    ADD CONSTRAINT ontology_terms_pkey PRIMARY KEY (ontology_term_id);


ALTER TABLE ONLY rnacen.pipeline_tracking_genome_mapping
    ADD CONSTRAINT pipeline_tracking_genome_mapping_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.pipeline_tracking_genome_mapping
    ADD CONSTRAINT pipeline_tracking_genome_mapping_urs_taxid_assembly_key UNIQUE (urs_taxid, assembly_id);


ALTER TABLE ONLY rnacen.pipeline_tracking_qa_scan
    ADD CONSTRAINT pipeline_tracking_scan_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.pipeline_tracking_traveler
    ADD CONSTRAINT pipeline_tracking_traveler_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.pipeline_tracking_traveler
    ADD CONSTRAINT pipeline_tracking_traveler_urs_key UNIQUE (urs);


ALTER TABLE ONLY rnacen.protein_info
    ADD CONSTRAINT protein_info_pkey PRIMARY KEY (protein_accession);


ALTER TABLE ONLY rnacen.publications
    ADD CONSTRAINT publications_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.qa_status
    ADD CONSTRAINT qa_status_pkey PRIMARY KEY (rna_id);


ALTER TABLE ONLY rnacen.release_stats
    ADD CONSTRAINT release_stats_pkey PRIMARY KEY (this_release);


ALTER TABLE ONLY rnacen.rfam_analyzed_sequences
    ADD CONSTRAINT rfam_analyzed_sequences_pkey PRIMARY KEY (upi);


ALTER TABLE ONLY rnacen.rfam_clans
    ADD CONSTRAINT rfam_clans_pkey PRIMARY KEY (rfam_clan_id);


ALTER TABLE ONLY rnacen.rfam_go_terms
    ADD CONSTRAINT rfam_go_terms_pkey PRIMARY KEY (rfam_go_term_id);


ALTER TABLE ONLY rnacen.rfam_model_hits
    ADD CONSTRAINT rfam_model_hits_pkey PRIMARY KEY (rfam_hit_id);


ALTER TABLE ONLY rnacen.rfam_models
    ADD CONSTRAINT rfam_models_pkey PRIMARY KEY (rfam_model_id);


ALTER TABLE ONLY rnacen.rna
    ADD CONSTRAINT rna_pkey PRIMARY KEY (upi);


ALTER TABLE ONLY rnacen.rnc_accession_sequence_region
    ADD CONSTRAINT rnc_accession_sequence_region_accession_region_id_key UNIQUE (accession, region_id);


ALTER TABLE ONLY rnacen.rnc_accession_sequence_region
    ADD CONSTRAINT rnc_accession_sequence_region_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.rnc_accessions
    ADD CONSTRAINT rnc_accessions_pkey PRIMARY KEY (accession);


ALTER TABLE ONLY rnacen.rnc_accession_sequence_feature
    ADD CONSTRAINT rnc_accessions_sequence_featu_accession_rnc_sequence_featur_key UNIQUE (accession, rnc_sequence_feature_id);


ALTER TABLE ONLY rnacen.rnc_accession_sequence_feature
    ADD CONSTRAINT rnc_accessions_sequence_features_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.rnc_chemical_components
    ADD CONSTRAINT rnc_chemical_components_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.cpat_results
    ADD CONSTRAINT rnc_cpat_results_pkey PRIMARY KEY (urs_taxid);


ALTER TABLE ONLY rnacen.rnc_database_json_stats
    ADD CONSTRAINT rnc_database_json_stats_pkey PRIMARY KEY (database);


ALTER TABLE ONLY rnacen.rnc_database
    ADD CONSTRAINT rnc_database_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.rnc_database_references
    ADD CONSTRAINT rnc_database_references_dbid_reference_id_key UNIQUE (dbid, reference_id);


ALTER TABLE ONLY rnacen.rnc_database_references
    ADD CONSTRAINT rnc_database_references_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.rnc_feedback_overlap
    ADD CONSTRAINT rnc_feedback_overlap_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.rnc_gene_status
    ADD CONSTRAINT rnc_gene_status_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.rnc_gene_status
    ADD CONSTRAINT rnc_gene_status_region_id_key UNIQUE (region_id);


ALTER TABLE ONLY rnacen.rnc_import_tracker
    ADD CONSTRAINT rnc_import_tracker_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.rnc_interactions
    ADD CONSTRAINT rnc_interactions_intact_id_key UNIQUE (intact_id);


ALTER TABLE ONLY rnacen.rnc_interactions
    ADD CONSTRAINT rnc_interactions_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.rnc_locus
    ADD CONSTRAINT rnc_locus_assembly_id_locus_name_key UNIQUE (assembly_id, locus_name);


ALTER TABLE ONLY rnacen.rnc_locus_members
    ADD CONSTRAINT rnc_locus_members_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.rnc_locus_members
    ADD CONSTRAINT rnc_locus_members_region_id_key UNIQUE (region_id);


ALTER TABLE ONLY rnacen.rnc_locus
    ADD CONSTRAINT rnc_locus_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.rnc_locus
    ADD CONSTRAINT rnc_locus_public_locus_name_key UNIQUE (public_locus_name);


ALTER TABLE ONLY rnacen.rnc_modifications
    ADD CONSTRAINT rnc_modifications_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.rnc_reference_map
    ADD CONSTRAINT rnc_reference_map_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.rnc_reference_map
    ADD CONSTRAINT "rnc_references_map$accession$reference_id" UNIQUE (accession, reference_id);


ALTER TABLE ONLY rnacen.rnc_references
    ADD CONSTRAINT rnc_references_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.rnc_related_sequences
    ADD CONSTRAINT rnc_related_sequences_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.rnc_relationship_types
    ADD CONSTRAINT rnc_relationship_types_pkey PRIMARY KEY (relationship_type);


ALTER TABLE ONLY rnacen.rnc_release
    ADD CONSTRAINT rnc_release_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.rnc_rna_precomputed
    ADD CONSTRAINT rnc_rna_precomputed_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.r2dt_models
    ADD CONSTRAINT rnc_secondary_structure_layout_models_model_name_key UNIQUE (model_name);


ALTER TABLE ONLY rnacen.r2dt_models
    ADD CONSTRAINT rnc_secondary_structure_layout_models_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.rnc_sequence_exons
    ADD CONSTRAINT rnc_sequence_exons_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.rnc_sequence_feature_types
    ADD CONSTRAINT rnc_sequence_feature_types_pkey PRIMARY KEY (feature_name);


ALTER TABLE ONLY rnacen.rnc_sequence_features
    ADD CONSTRAINT rnc_sequence_features_pkey PRIMARY KEY (rnc_sequence_features_id);


ALTER TABLE ONLY rnacen.rnc_sequence_regions
    ADD CONSTRAINT rnc_sequence_regions_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.rnc_taxonomy
    ADD CONSTRAINT rnc_taxonomy_pkey PRIMARY KEY (id);


ALTER TABLE ONLY rnacen.ensembl_pseudogene_exons
    ADD CONSTRAINT un_ensembl_pseduogene_exons__region_start_stop UNIQUE (region_id, exon_start, exon_stop);


ALTER TABLE ONLY rnacen.go_term_annotations
    ADD CONSTRAINT un_go_term_annotations UNIQUE (rna_id, qualifier, ontology_term_id, evidence_code, assigned_by);


ALTER TABLE ONLY rnacen.go_term_publication_map
    ADD CONSTRAINT un_go_term_publication_map_anno_ref UNIQUE (go_term_annotation_id, reference_id);


ALTER TABLE ONLY rnacen.r2dt_results
    ADD CONSTRAINT un_layout__urs UNIQUE (urs);


ALTER TABLE ONLY rnacen.rfam_go_terms
    ADD CONSTRAINT un_rfam_go_terms__rfam_model_id_ontology_term_id UNIQUE (rfam_model_id, ontology_term_id);


ALTER TABLE ONLY rnacen.rfam_model_hits
    ADD CONSTRAINT un_rfam_model_hits_unique_cols UNIQUE (sequence_start, sequence_stop, model_start, model_stop, rfam_model_id, upi);


ALTER TABLE ONLY rnacen.rnc_feedback_overlap
    ADD CONSTRAINT un_rnc_feedback_overlap__upi_assembly UNIQUE (upi_taxid, assembly_id);


ALTER TABLE ONLY rnacen.rnc_sequence_exons
    ADD CONSTRAINT un_rnc_sequence_exons__region_start_stop UNIQUE (region_id, exon_start, exon_stop);


ALTER TABLE ONLY rnacen.auth_permission
    ADD CONSTRAINT auth_content_type_id_508cf46651277a81_fk_django_content_type_id FOREIGN KEY (content_type_id) REFERENCES rnacen.django_content_type(id) DEFERRABLE INITIALLY DEFERRED;


ALTER TABLE ONLY rnacen.auth_group_permissions
    ADD CONSTRAINT auth_group_permissio_group_id_689710a9a73b7457_fk_auth_group_id FOREIGN KEY (group_id) REFERENCES rnacen.auth_group(id) DEFERRABLE INITIALLY DEFERRED;


ALTER TABLE ONLY rnacen.auth_group_permissions
    ADD CONSTRAINT auth_group_permission_id_1f49ccbbdc69d2fc_fk_auth_permission_id FOREIGN KEY (permission_id) REFERENCES rnacen.auth_permission(id) DEFERRABLE INITIALLY DEFERRED;


ALTER TABLE ONLY rnacen.auth_user_user_permissions
    ADD CONSTRAINT auth_user__permission_id_384b62483d7071f0_fk_auth_permission_id FOREIGN KEY (permission_id) REFERENCES rnacen.auth_permission(id) DEFERRABLE INITIALLY DEFERRED;


ALTER TABLE ONLY rnacen.auth_user_groups
    ADD CONSTRAINT auth_user_groups_group_id_33ac548dcf5f8e37_fk_auth_group_id FOREIGN KEY (group_id) REFERENCES rnacen.auth_group(id) DEFERRABLE INITIALLY DEFERRED;


ALTER TABLE ONLY rnacen.auth_user_groups
    ADD CONSTRAINT auth_user_groups_user_id_4b5ed4ffdb8fd9b0_fk_auth_user_id FOREIGN KEY (user_id) REFERENCES rnacen.auth_user(id) DEFERRABLE INITIALLY DEFERRED;


ALTER TABLE ONLY rnacen.auth_user_user_permissions
    ADD CONSTRAINT auth_user_user_permiss_user_id_7f0938558328534a_fk_auth_user_id FOREIGN KEY (user_id) REFERENCES rnacen.auth_user(id) DEFERRABLE INITIALLY DEFERRED;


ALTER TABLE ONLY rnacen.rnc_reference_map
    ADD CONSTRAINT ck_rnc_reference_map__reference_id FOREIGN KEY (reference_id) REFERENCES rnacen.rnc_references(id);


ALTER TABLE ONLY rnacen.django_admin_log
    ADD CONSTRAINT djan_content_type_id_697914295151027a_fk_django_content_type_id FOREIGN KEY (content_type_id) REFERENCES rnacen.django_content_type(id) DEFERRABLE INITIALLY DEFERRED;


ALTER TABLE ONLY rnacen.django_admin_log
    ADD CONSTRAINT django_admin_log_user_id_52fdd58701c5f563_fk_auth_user_id FOREIGN KEY (user_id) REFERENCES rnacen.auth_user(id) DEFERRABLE INITIALLY DEFERRED;


ALTER TABLE ONLY rnacen.ensembl_compara
    ADD CONSTRAINT ensembl_compara_urs_taxid_fkey FOREIGN KEY (urs_taxid) REFERENCES rnacen.rnc_rna_precomputed(id);


ALTER TABLE ONLY rnacen.ensembl_coordinate_systems
    ADD CONSTRAINT ensembl_coordinate_systems_assembly_id_fkey FOREIGN KEY (assembly_id) REFERENCES rnacen.ensembl_assembly(assembly_id) ON DELETE CASCADE;


ALTER TABLE ONLY rnacen.ensembl_karyotype
    ADD CONSTRAINT ensembl_karyotype_ensembl_assembly_assembly_id_fk FOREIGN KEY (assembly_id) REFERENCES rnacen.ensembl_assembly(assembly_id) ON DELETE CASCADE;


ALTER TABLE ONLY rnacen.ensembl_pseudogene_exons
    ADD CONSTRAINT ensembl_pseduogene_exons_region_id_fkey FOREIGN KEY (region_id) REFERENCES rnacen.ensembl_pseudogene_regions(id) ON DELETE CASCADE;


ALTER TABLE ONLY rnacen.ensembl_pseudogene_regions
    ADD CONSTRAINT ensembl_pseduogene_regions_assembly_id_fkey FOREIGN KEY (assembly_id) REFERENCES rnacen.ensembl_assembly(assembly_id) ON DELETE CASCADE;


ALTER TABLE ONLY rnacen.rnc_sequence_features
    ADD CONSTRAINT feature_provider_fk FOREIGN KEY (feature_provider) REFERENCES rnacen.rnc_sequence_feature_providers(name);


ALTER TABLE ONLY rnacen.r2dt_results
    ADD CONSTRAINT fk_layout__model_id FOREIGN KEY (model_id) REFERENCES rnacen.r2dt_models(id);


ALTER TABLE ONLY rnacen.r2dt_results
    ADD CONSTRAINT fk_layout__urs FOREIGN KEY (urs) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.cpat_results
    ADD CONSTRAINT fk_rnc_cpat_results__urs_taxid FOREIGN KEY (urs_taxid) REFERENCES rnacen.rnc_rna_precomputed(id);


ALTER TABLE ONLY rnacen.rnc_feedback_target_assemblies
    ADD CONSTRAINT fk_rnc_feedback_target_assemblies__assembly_id FOREIGN KEY (assembly_id) REFERENCES rnacen.ensembl_assembly(assembly_id) ON DELETE CASCADE;


ALTER TABLE ONLY rnacen.rnc_feedback_target_assemblies
    ADD CONSTRAINT fk_rnc_feedback_target_assemblies__dbid FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id) ON DELETE CASCADE;


ALTER TABLE ONLY rnacen.rnc_related_sequences
    ADD CONSTRAINT fk_rnc_related_sequences__relationship_type FOREIGN KEY (relationship_type) REFERENCES rnacen.rnc_relationship_types(relationship_type);


ALTER TABLE ONLY rnacen.rnc_sequence_features
    ADD CONSTRAINT fk_rnc_sequence_features__feature_type FOREIGN KEY (feature_name) REFERENCES rnacen.rnc_sequence_feature_types(feature_name);


ALTER TABLE ONLY rnacen.rnc_sequence_features
    ADD CONSTRAINT fk_rnc_sequence_features__taxid FOREIGN KEY (taxid) REFERENCES rnacen.rnc_taxonomy(id);


ALTER TABLE ONLY rnacen.go_term_annotations
    ADD CONSTRAINT go_term_annotations_evidence_code_fkey FOREIGN KEY (evidence_code) REFERENCES rnacen.ontology_terms(ontology_term_id);


ALTER TABLE ONLY rnacen.go_term_annotations
    ADD CONSTRAINT go_term_annotations_ontology_term_id_fkey FOREIGN KEY (ontology_term_id) REFERENCES rnacen.ontology_terms(ontology_term_id);


ALTER TABLE ONLY rnacen.go_term_publication_map
    ADD CONSTRAINT go_term_publication_map_go_term_annotation_id_fkey1 FOREIGN KEY (go_term_annotation_id) REFERENCES rnacen.go_term_annotations(go_term_annotation_id);


ALTER TABLE ONLY rnacen.go_term_publication_map
    ADD CONSTRAINT go_term_publication_map_reference_id_fkey FOREIGN KEY (reference_id) REFERENCES rnacen.rnc_references(id);


ALTER TABLE ONLY rnacen.insdc_so_term_mapping
    ADD CONSTRAINT insdc_so_term_mapping_so_term_id_fkey FOREIGN KEY (so_term_id) REFERENCES rnacen.ontology_terms(ontology_term_id);


ALTER TABLE ONLY rnacen.pipeline_tracking_genome_mapping
    ADD CONSTRAINT pipeline_tracking_genome_mapping_urs_taxid_fkey FOREIGN KEY (urs_taxid) REFERENCES rnacen.rnc_rna_precomputed(id);


ALTER TABLE ONLY rnacen.pipeline_tracking_qa_scan
    ADD CONSTRAINT pipeline_tracking_scan_urs_fkey FOREIGN KEY (urs) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.pipeline_tracking_traveler
    ADD CONSTRAINT pipeline_tracking_traveler_urs_fkey FOREIGN KEY (urs) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.qa_status
    ADD CONSTRAINT qa_status_upi_fkey FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.rfam_go_terms
    ADD CONSTRAINT rfa_rfam_model_id_418d94a91e74ecaa_fk_rfam_models_rfam_model_id FOREIGN KEY (rfam_model_id) REFERENCES rnacen.rfam_models(rfam_model_id) DEFERRABLE INITIALLY DEFERRED;


ALTER TABLE ONLY rnacen.rfam_model_hits
    ADD CONSTRAINT rfa_rfam_model_id_5088d5c42ad2571a_fk_rfam_models_rfam_model_id FOREIGN KEY (rfam_model_id) REFERENCES rnacen.rfam_models(rfam_model_id) DEFERRABLE INITIALLY DEFERRED;


ALTER TABLE ONLY rnacen.rfam_analyzed_sequences
    ADD CONSTRAINT rfam_analyzed_sequences_upi_143bc1f118b7f227_fk_rna_upi FOREIGN KEY (upi) REFERENCES rnacen.rna(upi) DEFERRABLE INITIALLY DEFERRED;


ALTER TABLE ONLY rnacen.rfam_go_terms
    ADD CONSTRAINT rfam_go_terms_ontology_term_id_fkey FOREIGN KEY (ontology_term_id) REFERENCES rnacen.ontology_terms(ontology_term_id);


ALTER TABLE ONLY rnacen.rfam_models
    ADD CONSTRAINT rfam_m_rfam_clan_id_4497ffd203fbfac6_fk_rfam_clans_rfam_clan_id FOREIGN KEY (rfam_clan_id) REFERENCES rnacen.rfam_clans(rfam_clan_id) DEFERRABLE INITIALLY DEFERRED;


ALTER TABLE ONLY rnacen.rfam_model_hits
    ADD CONSTRAINT rfam_model_hits_rnc_sequence_features_id_fkey FOREIGN KEY (rnc_sequence_features_id) REFERENCES rnacen.rnc_sequence_features(rnc_sequence_features_id);


ALTER TABLE ONLY rnacen.rfam_model_hits
    ADD CONSTRAINT rfam_model_hits_upi_4c2c11c85f9de4b0_fk_rna_upi FOREIGN KEY (upi) REFERENCES rnacen.rna(upi) DEFERRABLE INITIALLY DEFERRED;


ALTER TABLE ONLY rnacen.rfam_models
    ADD CONSTRAINT rfam_models_so_rna_type_fkey FOREIGN KEY (so_rna_type) REFERENCES rnacen.ontology_terms(ontology_term_id);


ALTER TABLE ONLY rnacen.rnc_accession_sequence_region
    ADD CONSTRAINT rnc_accession_sequence_region_accession_fkey FOREIGN KEY (accession) REFERENCES rnacen.rnc_accessions(accession);


ALTER TABLE ONLY rnacen.rnc_accession_sequence_region
    ADD CONSTRAINT rnc_accession_sequence_region_region_id_fkey FOREIGN KEY (region_id) REFERENCES rnacen.rnc_sequence_regions(id);


ALTER TABLE ONLY rnacen.rnc_accessions
    ADD CONSTRAINT rnc_accessions_rna_type_fkey FOREIGN KEY (rna_type) REFERENCES rnacen.ontology_terms(ontology_term_id);


ALTER TABLE ONLY rnacen.rnc_accession_sequence_feature
    ADD CONSTRAINT rnc_accessions_sequence_features_accession_fkey FOREIGN KEY (accession) REFERENCES rnacen.rnc_accessions(accession);


ALTER TABLE ONLY rnacen.rnc_accession_sequence_feature
    ADD CONSTRAINT rnc_accessions_sequence_features_rnc_sequence_feature_id_fkey FOREIGN KEY (rnc_sequence_feature_id) REFERENCES rnacen.rnc_sequence_features(rnc_sequence_features_id);


ALTER TABLE ONLY rnacen.rnc_database_references
    ADD CONSTRAINT rnc_database_references_dbid_fkey FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.rnc_database_references
    ADD CONSTRAINT rnc_database_references_reference_id_fkey FOREIGN KEY (reference_id) REFERENCES rnacen.rnc_references(id);


ALTER TABLE ONLY rnacen.rnc_feedback_overlap
    ADD CONSTRAINT rnc_feedback_overlap_assembly_id_fkey FOREIGN KEY (assembly_id) REFERENCES rnacen.ensembl_assembly(assembly_id) ON DELETE CASCADE;


ALTER TABLE ONLY rnacen.rnc_feedback_overlap
    ADD CONSTRAINT rnc_feedback_overlap_upi_taxid_fkey FOREIGN KEY (upi_taxid) REFERENCES rnacen.rnc_rna_precomputed(id);


ALTER TABLE ONLY rnacen.rnc_gene_status
    ADD CONSTRAINT rnc_gene_status_assembly_id_fkey FOREIGN KEY (assembly_id) REFERENCES rnacen.ensembl_assembly(assembly_id) ON DELETE CASCADE;


ALTER TABLE ONLY rnacen.rnc_gene_status
    ADD CONSTRAINT rnc_gene_status_region_id_fkey FOREIGN KEY (region_id) REFERENCES rnacen.rnc_sequence_regions(id) ON DELETE CASCADE;


ALTER TABLE ONLY rnacen.rnc_gene_status
    ADD CONSTRAINT rnc_gene_status_urs_taxid_fkey FOREIGN KEY (urs_taxid) REFERENCES rnacen.rnc_rna_precomputed(id) ON DELETE CASCADE;


ALTER TABLE ONLY rnacen.rnc_import_tracker
    ADD CONSTRAINT rnc_import_tracker_db_id_fkey FOREIGN KEY (db_id) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.rnc_interactions
    ADD CONSTRAINT rnc_interactions_urs_taxid_fkey FOREIGN KEY (urs_taxid) REFERENCES rnacen.rnc_rna_precomputed(id);


ALTER TABLE ONLY rnacen.rnc_locus
    ADD CONSTRAINT rnc_locus_assembly_id_fkey FOREIGN KEY (assembly_id) REFERENCES rnacen.ensembl_assembly(assembly_id) ON DELETE CASCADE;


ALTER TABLE ONLY rnacen.rnc_locus_members
    ADD CONSTRAINT rnc_locus_members_locus_id_fkey FOREIGN KEY (locus_id) REFERENCES rnacen.rnc_locus(id) ON DELETE CASCADE;


ALTER TABLE ONLY rnacen.rnc_locus_members
    ADD CONSTRAINT rnc_locus_members_region_id_fkey FOREIGN KEY (region_id) REFERENCES rnacen.rnc_sequence_regions(id) ON DELETE CASCADE;


ALTER TABLE ONLY rnacen.rnc_locus_members
    ADD CONSTRAINT rnc_locus_members_urs_taxid_fkey FOREIGN KEY (urs_taxid) REFERENCES rnacen.rnc_rna_precomputed(id) ON DELETE CASCADE;


ALTER TABLE ONLY rnacen.rnc_modifications
    ADD CONSTRAINT rnc_modifications_rnc_accessions__fk FOREIGN KEY (accession) REFERENCES rnacen.rnc_accessions(accession);


ALTER TABLE ONLY rnacen.rnc_related_sequences
    ADD CONSTRAINT rnc_related_sequences_source_accession_fkey FOREIGN KEY (source_accession) REFERENCES rnacen.rnc_accessions(accession);


ALTER TABLE ONLY rnacen.rnc_related_sequences
    ADD CONSTRAINT rnc_related_sequences_source_urs_taxid_fkey FOREIGN KEY (source_urs_taxid) REFERENCES rnacen.rnc_rna_precomputed(id);


ALTER TABLE ONLY rnacen.rnc_related_sequences
    ADD CONSTRAINT rnc_related_sequences_target_urs_taxid_fkey FOREIGN KEY (target_urs_taxid) REFERENCES rnacen.rnc_rna_precomputed(id);


ALTER TABLE ONLY rnacen.rnc_rna_precomputed
    ADD CONSTRAINT rnc_rna_precomputed_assigned_so_rna_type_fkey FOREIGN KEY (assigned_so_rna_type) REFERENCES rnacen.ontology_terms(ontology_term_id);


ALTER TABLE ONLY rnacen.rnc_rna_precomputed
    ADD CONSTRAINT rnc_rna_precomputed_last_release_fkey FOREIGN KEY (last_release) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.rnc_rna_precomputed
    ADD CONSTRAINT rnc_rna_precomputed_so_rna_type_fkey FOREIGN KEY (so_rna_type) REFERENCES rnacen.ontology_terms(ontology_term_id);


ALTER TABLE ONLY rnacen.r2dt_models
    ADD CONSTRAINT rnc_secondary_structure_layout_models_so_term_id_fkey FOREIGN KEY (so_term_id) REFERENCES rnacen.ontology_terms(ontology_term_id);


ALTER TABLE ONLY rnacen.r2dt_models
    ADD CONSTRAINT rnc_secondary_structure_layout_models_taxid_fkey FOREIGN KEY (taxid) REFERENCES rnacen.rnc_taxonomy(id);


ALTER TABLE ONLY rnacen.rnc_sequence_exons
    ADD CONSTRAINT rnc_sequence_exons_region_id_fkey FOREIGN KEY (region_id) REFERENCES rnacen.rnc_sequence_regions(id) ON DELETE CASCADE;


ALTER TABLE ONLY rnacen.rnc_sequence_features
    ADD CONSTRAINT rnc_sequence_features_accession_fkey FOREIGN KEY (accession) REFERENCES rnacen.rnc_accessions(accession);


ALTER TABLE ONLY rnacen.rnc_sequence_features
    ADD CONSTRAINT rnc_sequence_features_upi_fkey FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.rnc_sequence_regions
    ADD CONSTRAINT rnc_sequence_regions_assembly_id_fkey FOREIGN KEY (assembly_id) REFERENCES rnacen.ensembl_assembly(assembly_id) ON DELETE CASCADE;


ALTER TABLE ONLY rnacen.rnc_sequence_regions
    ADD CONSTRAINT rnc_sequence_regions_rnc_rna_precomputed__fk FOREIGN KEY (urs_taxid) REFERENCES rnacen.rnc_rna_precomputed(id) ON UPDATE CASCADE ON DELETE CASCADE;


ALTER TABLE ONLY rnacen.rnc_taxonomy
    ADD CONSTRAINT rnc_taxonomy_replaced_by_fkey FOREIGN KEY (replaced_by) REFERENCES rnacen.rnc_taxonomy(id);


ALTER TABLE ONLY rnacen.validate_layout_counts
    ADD CONSTRAINT validate_layout_counts_name_fkey FOREIGN KEY (name) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p10_deleted
    ADD CONSTRAINT xref_p10_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p10_deleted
    ADD CONSTRAINT xref_p10_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p10_deleted
    ADD CONSTRAINT xref_p10_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p10_deleted
    ADD CONSTRAINT xref_p10_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p10_not_deleted
    ADD CONSTRAINT xref_p10_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p10_not_deleted
    ADD CONSTRAINT xref_p10_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p10_not_deleted
    ADD CONSTRAINT xref_p10_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p10_not_deleted
    ADD CONSTRAINT xref_p10_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p11_deleted
    ADD CONSTRAINT xref_p11_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p11_deleted_old
    ADD CONSTRAINT xref_p11_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p11_deleted
    ADD CONSTRAINT xref_p11_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p11_deleted_old
    ADD CONSTRAINT xref_p11_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p11_deleted
    ADD CONSTRAINT xref_p11_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p11_deleted_old
    ADD CONSTRAINT xref_p11_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p11_deleted
    ADD CONSTRAINT xref_p11_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p11_deleted_old
    ADD CONSTRAINT xref_p11_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p11_not_deleted
    ADD CONSTRAINT xref_p11_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p11_not_deleted_old
    ADD CONSTRAINT xref_p11_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p11_not_deleted
    ADD CONSTRAINT xref_p11_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p11_not_deleted_old
    ADD CONSTRAINT xref_p11_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p11_not_deleted
    ADD CONSTRAINT xref_p11_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p11_not_deleted_old
    ADD CONSTRAINT xref_p11_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p11_not_deleted
    ADD CONSTRAINT xref_p11_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p11_not_deleted_old
    ADD CONSTRAINT xref_p11_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p12_deleted
    ADD CONSTRAINT xref_p12_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p12_deleted_old
    ADD CONSTRAINT xref_p12_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p12_deleted
    ADD CONSTRAINT xref_p12_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p12_deleted_old
    ADD CONSTRAINT xref_p12_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p12_deleted
    ADD CONSTRAINT xref_p12_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p12_deleted_old
    ADD CONSTRAINT xref_p12_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p12_deleted
    ADD CONSTRAINT xref_p12_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p12_deleted_old
    ADD CONSTRAINT xref_p12_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p12_not_deleted
    ADD CONSTRAINT xref_p12_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p12_not_deleted_old
    ADD CONSTRAINT xref_p12_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p12_not_deleted
    ADD CONSTRAINT xref_p12_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p12_not_deleted_old
    ADD CONSTRAINT xref_p12_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p12_not_deleted
    ADD CONSTRAINT xref_p12_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p12_not_deleted_old
    ADD CONSTRAINT xref_p12_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p12_not_deleted
    ADD CONSTRAINT xref_p12_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p12_not_deleted_old
    ADD CONSTRAINT xref_p12_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p13_deleted
    ADD CONSTRAINT xref_p13_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p13_deleted
    ADD CONSTRAINT xref_p13_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p13_deleted
    ADD CONSTRAINT xref_p13_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p13_deleted
    ADD CONSTRAINT xref_p13_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p13_not_deleted
    ADD CONSTRAINT xref_p13_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p13_not_deleted
    ADD CONSTRAINT xref_p13_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p13_not_deleted
    ADD CONSTRAINT xref_p13_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p13_not_deleted
    ADD CONSTRAINT xref_p13_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p14_deleted
    ADD CONSTRAINT xref_p14_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p14_deleted_old
    ADD CONSTRAINT xref_p14_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p14_deleted
    ADD CONSTRAINT xref_p14_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p14_deleted_old
    ADD CONSTRAINT xref_p14_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p14_deleted
    ADD CONSTRAINT xref_p14_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p14_deleted_old
    ADD CONSTRAINT xref_p14_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p14_deleted
    ADD CONSTRAINT xref_p14_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p14_deleted_old
    ADD CONSTRAINT xref_p14_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p14_not_deleted
    ADD CONSTRAINT xref_p14_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p14_not_deleted_old
    ADD CONSTRAINT xref_p14_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p14_not_deleted
    ADD CONSTRAINT xref_p14_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p14_not_deleted_old
    ADD CONSTRAINT xref_p14_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p14_not_deleted
    ADD CONSTRAINT xref_p14_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p14_not_deleted_old
    ADD CONSTRAINT xref_p14_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p14_not_deleted
    ADD CONSTRAINT xref_p14_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p14_not_deleted_old
    ADD CONSTRAINT xref_p14_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p15_deleted
    ADD CONSTRAINT xref_p15_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p15_deleted_old
    ADD CONSTRAINT xref_p15_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p15_deleted
    ADD CONSTRAINT xref_p15_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p15_deleted_old
    ADD CONSTRAINT xref_p15_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p15_deleted
    ADD CONSTRAINT xref_p15_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p15_deleted_old
    ADD CONSTRAINT xref_p15_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p15_deleted
    ADD CONSTRAINT xref_p15_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p15_deleted_old
    ADD CONSTRAINT xref_p15_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p15_not_deleted
    ADD CONSTRAINT xref_p15_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p15_not_deleted_old
    ADD CONSTRAINT xref_p15_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p15_not_deleted
    ADD CONSTRAINT xref_p15_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p15_not_deleted_old
    ADD CONSTRAINT xref_p15_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p15_not_deleted
    ADD CONSTRAINT xref_p15_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p15_not_deleted_old
    ADD CONSTRAINT xref_p15_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p15_not_deleted
    ADD CONSTRAINT xref_p15_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p15_not_deleted_old
    ADD CONSTRAINT xref_p15_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p16_deleted
    ADD CONSTRAINT xref_p16_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p16_deleted_old
    ADD CONSTRAINT xref_p16_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p16_deleted
    ADD CONSTRAINT xref_p16_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p16_deleted_old
    ADD CONSTRAINT xref_p16_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p16_deleted
    ADD CONSTRAINT xref_p16_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p16_deleted_old
    ADD CONSTRAINT xref_p16_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p16_deleted
    ADD CONSTRAINT xref_p16_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p16_deleted_old
    ADD CONSTRAINT xref_p16_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p16_not_deleted
    ADD CONSTRAINT xref_p16_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p16_not_deleted_old
    ADD CONSTRAINT xref_p16_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p16_not_deleted
    ADD CONSTRAINT xref_p16_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p16_not_deleted_old
    ADD CONSTRAINT xref_p16_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p16_not_deleted
    ADD CONSTRAINT xref_p16_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p16_not_deleted_old
    ADD CONSTRAINT xref_p16_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p16_not_deleted
    ADD CONSTRAINT xref_p16_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p16_not_deleted_old
    ADD CONSTRAINT xref_p16_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p17_deleted
    ADD CONSTRAINT xref_p17_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p17_deleted_old
    ADD CONSTRAINT xref_p17_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p17_deleted
    ADD CONSTRAINT xref_p17_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p17_deleted_old
    ADD CONSTRAINT xref_p17_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p17_deleted
    ADD CONSTRAINT xref_p17_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p17_deleted_old
    ADD CONSTRAINT xref_p17_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p17_deleted
    ADD CONSTRAINT xref_p17_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p17_deleted_old
    ADD CONSTRAINT xref_p17_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p17_not_deleted
    ADD CONSTRAINT xref_p17_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p17_not_deleted_old
    ADD CONSTRAINT xref_p17_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p17_not_deleted
    ADD CONSTRAINT xref_p17_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p17_not_deleted_old
    ADD CONSTRAINT xref_p17_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p17_not_deleted
    ADD CONSTRAINT xref_p17_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p17_not_deleted_old
    ADD CONSTRAINT xref_p17_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p17_not_deleted
    ADD CONSTRAINT xref_p17_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p17_not_deleted_old
    ADD CONSTRAINT xref_p17_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p18_deleted
    ADD CONSTRAINT xref_p18_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p18_deleted_old
    ADD CONSTRAINT xref_p18_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p18_deleted
    ADD CONSTRAINT xref_p18_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p18_deleted_old
    ADD CONSTRAINT xref_p18_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p18_deleted
    ADD CONSTRAINT xref_p18_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p18_deleted_old
    ADD CONSTRAINT xref_p18_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p18_deleted
    ADD CONSTRAINT xref_p18_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p18_deleted_old
    ADD CONSTRAINT xref_p18_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p18_not_deleted
    ADD CONSTRAINT xref_p18_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p18_not_deleted_old
    ADD CONSTRAINT xref_p18_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p18_not_deleted
    ADD CONSTRAINT xref_p18_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p18_not_deleted_old
    ADD CONSTRAINT xref_p18_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p18_not_deleted
    ADD CONSTRAINT xref_p18_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p18_not_deleted_old
    ADD CONSTRAINT xref_p18_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p18_not_deleted
    ADD CONSTRAINT xref_p18_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p18_not_deleted_old
    ADD CONSTRAINT xref_p18_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p19_deleted
    ADD CONSTRAINT xref_p19_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p19_deleted
    ADD CONSTRAINT xref_p19_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p19_deleted
    ADD CONSTRAINT xref_p19_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p19_deleted
    ADD CONSTRAINT xref_p19_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p19_not_deleted
    ADD CONSTRAINT xref_p19_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p19_not_deleted
    ADD CONSTRAINT xref_p19_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p19_not_deleted
    ADD CONSTRAINT xref_p19_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p19_not_deleted
    ADD CONSTRAINT xref_p19_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p1_deleted
    ADD CONSTRAINT xref_p1_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p1_deleted_old
    ADD CONSTRAINT xref_p1_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p1_deleted
    ADD CONSTRAINT xref_p1_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p1_deleted_old
    ADD CONSTRAINT xref_p1_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p1_deleted
    ADD CONSTRAINT xref_p1_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p1_deleted_old
    ADD CONSTRAINT xref_p1_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p1_deleted
    ADD CONSTRAINT xref_p1_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p1_deleted_old
    ADD CONSTRAINT xref_p1_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p1_not_deleted
    ADD CONSTRAINT xref_p1_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p1_not_deleted_old
    ADD CONSTRAINT xref_p1_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p1_not_deleted
    ADD CONSTRAINT xref_p1_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p1_not_deleted_old
    ADD CONSTRAINT xref_p1_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p1_not_deleted
    ADD CONSTRAINT xref_p1_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p1_not_deleted_old
    ADD CONSTRAINT xref_p1_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p1_not_deleted
    ADD CONSTRAINT xref_p1_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p1_not_deleted_old
    ADD CONSTRAINT xref_p1_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p20_deleted
    ADD CONSTRAINT xref_p20_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p20_deleted_old
    ADD CONSTRAINT xref_p20_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p20_deleted
    ADD CONSTRAINT xref_p20_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p20_deleted_old
    ADD CONSTRAINT xref_p20_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p20_deleted
    ADD CONSTRAINT xref_p20_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p20_deleted_old
    ADD CONSTRAINT xref_p20_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p20_deleted
    ADD CONSTRAINT xref_p20_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p20_deleted_old
    ADD CONSTRAINT xref_p20_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p20_not_deleted
    ADD CONSTRAINT xref_p20_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p20_not_deleted_old
    ADD CONSTRAINT xref_p20_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p20_not_deleted
    ADD CONSTRAINT xref_p20_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p20_not_deleted_old
    ADD CONSTRAINT xref_p20_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p20_not_deleted
    ADD CONSTRAINT xref_p20_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p20_not_deleted_old
    ADD CONSTRAINT xref_p20_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p20_not_deleted
    ADD CONSTRAINT xref_p20_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p20_not_deleted_old
    ADD CONSTRAINT xref_p20_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p21_deleted
    ADD CONSTRAINT xref_p21_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p21_deleted
    ADD CONSTRAINT xref_p21_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p21_deleted
    ADD CONSTRAINT xref_p21_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p21_deleted
    ADD CONSTRAINT xref_p21_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p21_not_deleted
    ADD CONSTRAINT xref_p21_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p21_not_deleted
    ADD CONSTRAINT xref_p21_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p21_not_deleted
    ADD CONSTRAINT xref_p21_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p21_not_deleted
    ADD CONSTRAINT xref_p21_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p22_deleted
    ADD CONSTRAINT xref_p22_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p22_deleted
    ADD CONSTRAINT xref_p22_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p22_deleted
    ADD CONSTRAINT xref_p22_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p22_deleted
    ADD CONSTRAINT xref_p22_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p22_not_deleted
    ADD CONSTRAINT xref_p22_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p22_not_deleted
    ADD CONSTRAINT xref_p22_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p22_not_deleted
    ADD CONSTRAINT xref_p22_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p22_not_deleted
    ADD CONSTRAINT xref_p22_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p23_deleted
    ADD CONSTRAINT xref_p23_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p23_deleted_old
    ADD CONSTRAINT xref_p23_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p23_deleted
    ADD CONSTRAINT xref_p23_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p23_deleted_old
    ADD CONSTRAINT xref_p23_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p23_deleted
    ADD CONSTRAINT xref_p23_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p23_deleted_old
    ADD CONSTRAINT xref_p23_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p23_deleted
    ADD CONSTRAINT xref_p23_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p23_deleted_old
    ADD CONSTRAINT xref_p23_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p23_not_deleted
    ADD CONSTRAINT xref_p23_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p23_not_deleted_old
    ADD CONSTRAINT xref_p23_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p23_not_deleted
    ADD CONSTRAINT xref_p23_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p23_not_deleted_old
    ADD CONSTRAINT xref_p23_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p23_not_deleted
    ADD CONSTRAINT xref_p23_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p23_not_deleted_old
    ADD CONSTRAINT xref_p23_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p23_not_deleted
    ADD CONSTRAINT xref_p23_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p23_not_deleted_old
    ADD CONSTRAINT xref_p23_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p24_deleted
    ADD CONSTRAINT xref_p24_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p24_deleted_old
    ADD CONSTRAINT xref_p24_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p24_deleted
    ADD CONSTRAINT xref_p24_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p24_deleted_old
    ADD CONSTRAINT xref_p24_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p24_deleted
    ADD CONSTRAINT xref_p24_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p24_deleted_old
    ADD CONSTRAINT xref_p24_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p24_deleted
    ADD CONSTRAINT xref_p24_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p24_deleted_old
    ADD CONSTRAINT xref_p24_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p24_not_deleted
    ADD CONSTRAINT xref_p24_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p24_not_deleted_old
    ADD CONSTRAINT xref_p24_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p24_not_deleted
    ADD CONSTRAINT xref_p24_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p24_not_deleted_old
    ADD CONSTRAINT xref_p24_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p24_not_deleted
    ADD CONSTRAINT xref_p24_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p24_not_deleted_old
    ADD CONSTRAINT xref_p24_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p24_not_deleted
    ADD CONSTRAINT xref_p24_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p24_not_deleted_old
    ADD CONSTRAINT xref_p24_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p25_deleted
    ADD CONSTRAINT xref_p25_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p25_deleted_old
    ADD CONSTRAINT xref_p25_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p25_deleted
    ADD CONSTRAINT xref_p25_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p25_deleted_old
    ADD CONSTRAINT xref_p25_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p25_deleted
    ADD CONSTRAINT xref_p25_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p25_deleted_old
    ADD CONSTRAINT xref_p25_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p25_deleted
    ADD CONSTRAINT xref_p25_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p25_deleted_old
    ADD CONSTRAINT xref_p25_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p25_not_deleted
    ADD CONSTRAINT xref_p25_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p25_not_deleted_old
    ADD CONSTRAINT xref_p25_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p25_not_deleted
    ADD CONSTRAINT xref_p25_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p25_not_deleted_old
    ADD CONSTRAINT xref_p25_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p25_not_deleted
    ADD CONSTRAINT xref_p25_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p25_not_deleted_old
    ADD CONSTRAINT xref_p25_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p25_not_deleted
    ADD CONSTRAINT xref_p25_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p25_not_deleted_old
    ADD CONSTRAINT xref_p25_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p26_deleted
    ADD CONSTRAINT xref_p26_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p26_deleted_old
    ADD CONSTRAINT xref_p26_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p26_deleted
    ADD CONSTRAINT xref_p26_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p26_deleted_old
    ADD CONSTRAINT xref_p26_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p26_deleted
    ADD CONSTRAINT xref_p26_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p26_deleted_old
    ADD CONSTRAINT xref_p26_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p26_deleted
    ADD CONSTRAINT xref_p26_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p26_deleted_old
    ADD CONSTRAINT xref_p26_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p26_not_deleted
    ADD CONSTRAINT xref_p26_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p26_not_deleted_old
    ADD CONSTRAINT xref_p26_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p26_not_deleted
    ADD CONSTRAINT xref_p26_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p26_not_deleted_old
    ADD CONSTRAINT xref_p26_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p26_not_deleted
    ADD CONSTRAINT xref_p26_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p26_not_deleted_old
    ADD CONSTRAINT xref_p26_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p26_not_deleted
    ADD CONSTRAINT xref_p26_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p26_not_deleted_old
    ADD CONSTRAINT xref_p26_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p27_deleted
    ADD CONSTRAINT xref_p27_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p27_deleted
    ADD CONSTRAINT xref_p27_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p27_deleted
    ADD CONSTRAINT xref_p27_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p27_deleted
    ADD CONSTRAINT xref_p27_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p27_not_deleted
    ADD CONSTRAINT xref_p27_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p27_not_deleted
    ADD CONSTRAINT xref_p27_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p27_not_deleted
    ADD CONSTRAINT xref_p27_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p27_not_deleted
    ADD CONSTRAINT xref_p27_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p28_deleted
    ADD CONSTRAINT xref_p28_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p28_deleted
    ADD CONSTRAINT xref_p28_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p28_deleted
    ADD CONSTRAINT xref_p28_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p28_deleted
    ADD CONSTRAINT xref_p28_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p28_not_deleted
    ADD CONSTRAINT xref_p28_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p28_not_deleted
    ADD CONSTRAINT xref_p28_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p28_not_deleted
    ADD CONSTRAINT xref_p28_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p28_not_deleted
    ADD CONSTRAINT xref_p28_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p29_deleted
    ADD CONSTRAINT xref_p29_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p29_deleted
    ADD CONSTRAINT xref_p29_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p29_deleted
    ADD CONSTRAINT xref_p29_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p29_deleted
    ADD CONSTRAINT xref_p29_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p29_not_deleted
    ADD CONSTRAINT xref_p29_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p29_not_deleted
    ADD CONSTRAINT xref_p29_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p29_not_deleted
    ADD CONSTRAINT xref_p29_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p29_not_deleted
    ADD CONSTRAINT xref_p29_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p2_deleted
    ADD CONSTRAINT xref_p2_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p2_deleted_old
    ADD CONSTRAINT xref_p2_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p2_deleted
    ADD CONSTRAINT xref_p2_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p2_deleted_old
    ADD CONSTRAINT xref_p2_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p2_deleted
    ADD CONSTRAINT xref_p2_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p2_deleted_old
    ADD CONSTRAINT xref_p2_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p2_deleted
    ADD CONSTRAINT xref_p2_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p2_deleted_old
    ADD CONSTRAINT xref_p2_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p2_not_deleted
    ADD CONSTRAINT xref_p2_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p2_not_deleted_old
    ADD CONSTRAINT xref_p2_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p2_not_deleted
    ADD CONSTRAINT xref_p2_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p2_not_deleted_old
    ADD CONSTRAINT xref_p2_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p2_not_deleted
    ADD CONSTRAINT xref_p2_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p2_not_deleted_old
    ADD CONSTRAINT xref_p2_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p2_not_deleted
    ADD CONSTRAINT xref_p2_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p2_not_deleted_old
    ADD CONSTRAINT xref_p2_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p30_deleted
    ADD CONSTRAINT xref_p30_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p30_deleted_old
    ADD CONSTRAINT xref_p30_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p30_deleted
    ADD CONSTRAINT xref_p30_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p30_deleted_old
    ADD CONSTRAINT xref_p30_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p30_deleted
    ADD CONSTRAINT xref_p30_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p30_deleted_old
    ADD CONSTRAINT xref_p30_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p30_deleted
    ADD CONSTRAINT xref_p30_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p30_deleted_old
    ADD CONSTRAINT xref_p30_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p30_not_deleted
    ADD CONSTRAINT xref_p30_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p30_not_deleted_old
    ADD CONSTRAINT xref_p30_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p30_not_deleted
    ADD CONSTRAINT xref_p30_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p30_not_deleted_old
    ADD CONSTRAINT xref_p30_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p30_not_deleted
    ADD CONSTRAINT xref_p30_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p30_not_deleted_old
    ADD CONSTRAINT xref_p30_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p30_not_deleted
    ADD CONSTRAINT xref_p30_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p30_not_deleted_old
    ADD CONSTRAINT xref_p30_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p31_deleted
    ADD CONSTRAINT xref_p31_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p31_deleted_old
    ADD CONSTRAINT xref_p31_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p31_deleted
    ADD CONSTRAINT xref_p31_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p31_deleted_old
    ADD CONSTRAINT xref_p31_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p31_deleted
    ADD CONSTRAINT xref_p31_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p31_deleted_old
    ADD CONSTRAINT xref_p31_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p31_deleted
    ADD CONSTRAINT xref_p31_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p31_deleted_old
    ADD CONSTRAINT xref_p31_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p31_not_deleted
    ADD CONSTRAINT xref_p31_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p31_not_deleted_old
    ADD CONSTRAINT xref_p31_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p31_not_deleted
    ADD CONSTRAINT xref_p31_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p31_not_deleted_old
    ADD CONSTRAINT xref_p31_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p31_not_deleted
    ADD CONSTRAINT xref_p31_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p31_not_deleted_old
    ADD CONSTRAINT xref_p31_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p31_not_deleted
    ADD CONSTRAINT xref_p31_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p31_not_deleted_old
    ADD CONSTRAINT xref_p31_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p32_deleted
    ADD CONSTRAINT xref_p32_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p32_deleted
    ADD CONSTRAINT xref_p32_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p32_deleted
    ADD CONSTRAINT xref_p32_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p32_deleted
    ADD CONSTRAINT xref_p32_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p32_not_deleted
    ADD CONSTRAINT xref_p32_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p32_not_deleted
    ADD CONSTRAINT xref_p32_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p32_not_deleted
    ADD CONSTRAINT xref_p32_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p32_not_deleted
    ADD CONSTRAINT xref_p32_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p33_deleted
    ADD CONSTRAINT xref_p33_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p33_deleted_old
    ADD CONSTRAINT xref_p33_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p33_deleted
    ADD CONSTRAINT xref_p33_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p33_deleted_old
    ADD CONSTRAINT xref_p33_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p33_deleted
    ADD CONSTRAINT xref_p33_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p33_deleted_old
    ADD CONSTRAINT xref_p33_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p33_deleted
    ADD CONSTRAINT xref_p33_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p33_deleted_old
    ADD CONSTRAINT xref_p33_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p33_not_deleted
    ADD CONSTRAINT xref_p33_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p33_not_deleted_old
    ADD CONSTRAINT xref_p33_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p33_not_deleted
    ADD CONSTRAINT xref_p33_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p33_not_deleted_old
    ADD CONSTRAINT xref_p33_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p33_not_deleted
    ADD CONSTRAINT xref_p33_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p33_not_deleted_old
    ADD CONSTRAINT xref_p33_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p33_not_deleted
    ADD CONSTRAINT xref_p33_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p33_not_deleted_old
    ADD CONSTRAINT xref_p33_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p34_deleted
    ADD CONSTRAINT xref_p34_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p34_deleted_old
    ADD CONSTRAINT xref_p34_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p34_deleted
    ADD CONSTRAINT xref_p34_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p34_deleted_old
    ADD CONSTRAINT xref_p34_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p34_deleted
    ADD CONSTRAINT xref_p34_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p34_deleted_old
    ADD CONSTRAINT xref_p34_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p34_deleted
    ADD CONSTRAINT xref_p34_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p34_deleted_old
    ADD CONSTRAINT xref_p34_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p34_not_deleted
    ADD CONSTRAINT xref_p34_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p34_not_deleted_old
    ADD CONSTRAINT xref_p34_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p34_not_deleted
    ADD CONSTRAINT xref_p34_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p34_not_deleted_old
    ADD CONSTRAINT xref_p34_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p34_not_deleted
    ADD CONSTRAINT xref_p34_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p34_not_deleted_old
    ADD CONSTRAINT xref_p34_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p34_not_deleted
    ADD CONSTRAINT xref_p34_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p34_not_deleted_old
    ADD CONSTRAINT xref_p34_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p35_deleted
    ADD CONSTRAINT xref_p35_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p35_deleted_old
    ADD CONSTRAINT xref_p35_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p35_deleted
    ADD CONSTRAINT xref_p35_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p35_deleted_old
    ADD CONSTRAINT xref_p35_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p35_deleted
    ADD CONSTRAINT xref_p35_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p35_deleted_old
    ADD CONSTRAINT xref_p35_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p35_deleted
    ADD CONSTRAINT xref_p35_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p35_deleted_old
    ADD CONSTRAINT xref_p35_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p35_not_deleted
    ADD CONSTRAINT xref_p35_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p35_not_deleted_old
    ADD CONSTRAINT xref_p35_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p35_not_deleted
    ADD CONSTRAINT xref_p35_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p35_not_deleted_old
    ADD CONSTRAINT xref_p35_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p35_not_deleted
    ADD CONSTRAINT xref_p35_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p35_not_deleted_old
    ADD CONSTRAINT xref_p35_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p35_not_deleted
    ADD CONSTRAINT xref_p35_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p35_not_deleted_old
    ADD CONSTRAINT xref_p35_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p36_deleted
    ADD CONSTRAINT xref_p36_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p36_deleted_old
    ADD CONSTRAINT xref_p36_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p36_deleted
    ADD CONSTRAINT xref_p36_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p36_deleted_old
    ADD CONSTRAINT xref_p36_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p36_deleted
    ADD CONSTRAINT xref_p36_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p36_deleted_old
    ADD CONSTRAINT xref_p36_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p36_deleted
    ADD CONSTRAINT xref_p36_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p36_deleted_old
    ADD CONSTRAINT xref_p36_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p36_not_deleted
    ADD CONSTRAINT xref_p36_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p36_not_deleted_old
    ADD CONSTRAINT xref_p36_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p36_not_deleted
    ADD CONSTRAINT xref_p36_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p36_not_deleted_old
    ADD CONSTRAINT xref_p36_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p36_not_deleted
    ADD CONSTRAINT xref_p36_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p36_not_deleted_old
    ADD CONSTRAINT xref_p36_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p36_not_deleted
    ADD CONSTRAINT xref_p36_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p36_not_deleted_old
    ADD CONSTRAINT xref_p36_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p37_deleted
    ADD CONSTRAINT xref_p37_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p37_deleted_old
    ADD CONSTRAINT xref_p37_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p37_deleted
    ADD CONSTRAINT xref_p37_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p37_deleted_old
    ADD CONSTRAINT xref_p37_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p37_deleted
    ADD CONSTRAINT xref_p37_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p37_deleted_old
    ADD CONSTRAINT xref_p37_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p37_deleted
    ADD CONSTRAINT xref_p37_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p37_deleted_old
    ADD CONSTRAINT xref_p37_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p37_not_deleted
    ADD CONSTRAINT xref_p37_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p37_not_deleted_old
    ADD CONSTRAINT xref_p37_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p37_not_deleted
    ADD CONSTRAINT xref_p37_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p37_not_deleted_old
    ADD CONSTRAINT xref_p37_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p37_not_deleted
    ADD CONSTRAINT xref_p37_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p37_not_deleted_old
    ADD CONSTRAINT xref_p37_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p37_not_deleted
    ADD CONSTRAINT xref_p37_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p37_not_deleted_old
    ADD CONSTRAINT xref_p37_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p38_deleted
    ADD CONSTRAINT xref_p38_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p38_deleted_old
    ADD CONSTRAINT xref_p38_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p38_deleted
    ADD CONSTRAINT xref_p38_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p38_deleted_old
    ADD CONSTRAINT xref_p38_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p38_deleted
    ADD CONSTRAINT xref_p38_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p38_deleted_old
    ADD CONSTRAINT xref_p38_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p38_deleted
    ADD CONSTRAINT xref_p38_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p38_deleted_old
    ADD CONSTRAINT xref_p38_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p38_not_deleted
    ADD CONSTRAINT xref_p38_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p38_not_deleted_old
    ADD CONSTRAINT xref_p38_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p38_not_deleted
    ADD CONSTRAINT xref_p38_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p38_not_deleted_old
    ADD CONSTRAINT xref_p38_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p38_not_deleted
    ADD CONSTRAINT xref_p38_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p38_not_deleted_old
    ADD CONSTRAINT xref_p38_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p38_not_deleted
    ADD CONSTRAINT xref_p38_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p38_not_deleted_old
    ADD CONSTRAINT xref_p38_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p39_deleted
    ADD CONSTRAINT xref_p39_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p39_deleted_old
    ADD CONSTRAINT xref_p39_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p39_deleted
    ADD CONSTRAINT xref_p39_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p39_deleted_old
    ADD CONSTRAINT xref_p39_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p39_deleted
    ADD CONSTRAINT xref_p39_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p39_deleted_old
    ADD CONSTRAINT xref_p39_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p39_deleted
    ADD CONSTRAINT xref_p39_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p39_deleted_old
    ADD CONSTRAINT xref_p39_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p39_not_deleted
    ADD CONSTRAINT xref_p39_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p39_not_deleted_old
    ADD CONSTRAINT xref_p39_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p39_not_deleted
    ADD CONSTRAINT xref_p39_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p39_not_deleted_old
    ADD CONSTRAINT xref_p39_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p39_not_deleted
    ADD CONSTRAINT xref_p39_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p39_not_deleted_old
    ADD CONSTRAINT xref_p39_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p39_not_deleted
    ADD CONSTRAINT xref_p39_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p39_not_deleted_old
    ADD CONSTRAINT xref_p39_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p3_deleted
    ADD CONSTRAINT xref_p3_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p3_deleted_old
    ADD CONSTRAINT xref_p3_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p3_deleted
    ADD CONSTRAINT xref_p3_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p3_deleted_old
    ADD CONSTRAINT xref_p3_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p3_deleted
    ADD CONSTRAINT xref_p3_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p3_deleted_old
    ADD CONSTRAINT xref_p3_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p3_deleted
    ADD CONSTRAINT xref_p3_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p3_deleted_old
    ADD CONSTRAINT xref_p3_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p3_not_deleted
    ADD CONSTRAINT xref_p3_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p3_not_deleted_old
    ADD CONSTRAINT xref_p3_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p3_not_deleted
    ADD CONSTRAINT xref_p3_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p3_not_deleted_old
    ADD CONSTRAINT xref_p3_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p3_not_deleted
    ADD CONSTRAINT xref_p3_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p3_not_deleted_old
    ADD CONSTRAINT xref_p3_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p3_not_deleted
    ADD CONSTRAINT xref_p3_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p3_not_deleted_old
    ADD CONSTRAINT xref_p3_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p40_deleted
    ADD CONSTRAINT xref_p40_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p40_deleted_old
    ADD CONSTRAINT xref_p40_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p40_deleted
    ADD CONSTRAINT xref_p40_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p40_deleted_old
    ADD CONSTRAINT xref_p40_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p40_deleted
    ADD CONSTRAINT xref_p40_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p40_deleted_old
    ADD CONSTRAINT xref_p40_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p40_deleted
    ADD CONSTRAINT xref_p40_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p40_deleted_old
    ADD CONSTRAINT xref_p40_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p40_not_deleted
    ADD CONSTRAINT xref_p40_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p40_not_deleted_old
    ADD CONSTRAINT xref_p40_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p40_not_deleted
    ADD CONSTRAINT xref_p40_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p40_not_deleted_old
    ADD CONSTRAINT xref_p40_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p40_not_deleted
    ADD CONSTRAINT xref_p40_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p40_not_deleted_old
    ADD CONSTRAINT xref_p40_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p40_not_deleted
    ADD CONSTRAINT xref_p40_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p40_not_deleted_old
    ADD CONSTRAINT xref_p40_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p41_deleted
    ADD CONSTRAINT xref_p41_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p41_deleted_old
    ADD CONSTRAINT xref_p41_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p41_deleted
    ADD CONSTRAINT xref_p41_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p41_deleted_old
    ADD CONSTRAINT xref_p41_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p41_deleted
    ADD CONSTRAINT xref_p41_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p41_deleted_old
    ADD CONSTRAINT xref_p41_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p41_deleted
    ADD CONSTRAINT xref_p41_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p41_deleted_old
    ADD CONSTRAINT xref_p41_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p41_not_deleted
    ADD CONSTRAINT xref_p41_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p41_not_deleted_old
    ADD CONSTRAINT xref_p41_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p41_not_deleted
    ADD CONSTRAINT xref_p41_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p41_not_deleted_old
    ADD CONSTRAINT xref_p41_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p41_not_deleted
    ADD CONSTRAINT xref_p41_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p41_not_deleted_old
    ADD CONSTRAINT xref_p41_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p41_not_deleted
    ADD CONSTRAINT xref_p41_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p41_not_deleted_old
    ADD CONSTRAINT xref_p41_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p42_deleted
    ADD CONSTRAINT xref_p42_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p42_deleted_old
    ADD CONSTRAINT xref_p42_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p42_deleted
    ADD CONSTRAINT xref_p42_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p42_deleted_old
    ADD CONSTRAINT xref_p42_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p42_deleted
    ADD CONSTRAINT xref_p42_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p42_deleted_old
    ADD CONSTRAINT xref_p42_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p42_deleted
    ADD CONSTRAINT xref_p42_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p42_deleted_old
    ADD CONSTRAINT xref_p42_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p42_not_deleted
    ADD CONSTRAINT xref_p42_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p42_not_deleted_old
    ADD CONSTRAINT xref_p42_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p42_not_deleted
    ADD CONSTRAINT xref_p42_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p42_not_deleted_old
    ADD CONSTRAINT xref_p42_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p42_not_deleted
    ADD CONSTRAINT xref_p42_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p42_not_deleted_old
    ADD CONSTRAINT xref_p42_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p42_not_deleted
    ADD CONSTRAINT xref_p42_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p42_not_deleted_old
    ADD CONSTRAINT xref_p42_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p43_deleted
    ADD CONSTRAINT xref_p43_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p43_deleted_old
    ADD CONSTRAINT xref_p43_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p43_deleted
    ADD CONSTRAINT xref_p43_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p43_deleted_old
    ADD CONSTRAINT xref_p43_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p43_deleted
    ADD CONSTRAINT xref_p43_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p43_deleted_old
    ADD CONSTRAINT xref_p43_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p43_deleted
    ADD CONSTRAINT xref_p43_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p43_deleted_old
    ADD CONSTRAINT xref_p43_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p43_not_deleted
    ADD CONSTRAINT xref_p43_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p43_not_deleted_old
    ADD CONSTRAINT xref_p43_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p43_not_deleted
    ADD CONSTRAINT xref_p43_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p43_not_deleted_old
    ADD CONSTRAINT xref_p43_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p43_not_deleted
    ADD CONSTRAINT xref_p43_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p43_not_deleted_old
    ADD CONSTRAINT xref_p43_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p43_not_deleted
    ADD CONSTRAINT xref_p43_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p43_not_deleted_old
    ADD CONSTRAINT xref_p43_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p44_deleted
    ADD CONSTRAINT xref_p44_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p44_deleted_old
    ADD CONSTRAINT xref_p44_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p44_deleted
    ADD CONSTRAINT xref_p44_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p44_deleted_old
    ADD CONSTRAINT xref_p44_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p44_deleted
    ADD CONSTRAINT xref_p44_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p44_deleted_old
    ADD CONSTRAINT xref_p44_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p44_deleted
    ADD CONSTRAINT xref_p44_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p44_deleted_old
    ADD CONSTRAINT xref_p44_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p44_not_deleted
    ADD CONSTRAINT xref_p44_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p44_not_deleted_old
    ADD CONSTRAINT xref_p44_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p44_not_deleted
    ADD CONSTRAINT xref_p44_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p44_not_deleted_old
    ADD CONSTRAINT xref_p44_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p44_not_deleted
    ADD CONSTRAINT xref_p44_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p44_not_deleted_old
    ADD CONSTRAINT xref_p44_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p44_not_deleted
    ADD CONSTRAINT xref_p44_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p44_not_deleted_old
    ADD CONSTRAINT xref_p44_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p45_deleted
    ADD CONSTRAINT xref_p45_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p45_deleted_old
    ADD CONSTRAINT xref_p45_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p45_deleted
    ADD CONSTRAINT xref_p45_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p45_deleted_old
    ADD CONSTRAINT xref_p45_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p45_deleted
    ADD CONSTRAINT xref_p45_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p45_deleted_old
    ADD CONSTRAINT xref_p45_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p45_deleted
    ADD CONSTRAINT xref_p45_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p45_deleted_old
    ADD CONSTRAINT xref_p45_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p45_not_deleted
    ADD CONSTRAINT xref_p45_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p45_not_deleted_old
    ADD CONSTRAINT xref_p45_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p45_not_deleted
    ADD CONSTRAINT xref_p45_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p45_not_deleted_old
    ADD CONSTRAINT xref_p45_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p45_not_deleted
    ADD CONSTRAINT xref_p45_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p45_not_deleted_old
    ADD CONSTRAINT xref_p45_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p45_not_deleted
    ADD CONSTRAINT xref_p45_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p45_not_deleted_old
    ADD CONSTRAINT xref_p45_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p46_deleted
    ADD CONSTRAINT xref_p46_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p46_deleted_old
    ADD CONSTRAINT xref_p46_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p46_deleted
    ADD CONSTRAINT xref_p46_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p46_deleted_old
    ADD CONSTRAINT xref_p46_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p46_deleted
    ADD CONSTRAINT xref_p46_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p46_deleted_old
    ADD CONSTRAINT xref_p46_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p46_deleted
    ADD CONSTRAINT xref_p46_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p46_deleted_old
    ADD CONSTRAINT xref_p46_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p46_not_deleted
    ADD CONSTRAINT xref_p46_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p46_not_deleted_old
    ADD CONSTRAINT xref_p46_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p46_not_deleted
    ADD CONSTRAINT xref_p46_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p46_not_deleted_old
    ADD CONSTRAINT xref_p46_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p46_not_deleted
    ADD CONSTRAINT xref_p46_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p46_not_deleted_old
    ADD CONSTRAINT xref_p46_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p46_not_deleted
    ADD CONSTRAINT xref_p46_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p46_not_deleted_old
    ADD CONSTRAINT xref_p46_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p47_deleted
    ADD CONSTRAINT xref_p47_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p47_deleted_old
    ADD CONSTRAINT xref_p47_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p47_deleted
    ADD CONSTRAINT xref_p47_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p47_deleted_old
    ADD CONSTRAINT xref_p47_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p47_deleted
    ADD CONSTRAINT xref_p47_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p47_deleted_old
    ADD CONSTRAINT xref_p47_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p47_deleted
    ADD CONSTRAINT xref_p47_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p47_deleted_old
    ADD CONSTRAINT xref_p47_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p47_not_deleted
    ADD CONSTRAINT xref_p47_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p47_not_deleted_old
    ADD CONSTRAINT xref_p47_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p47_not_deleted
    ADD CONSTRAINT xref_p47_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p47_not_deleted_old
    ADD CONSTRAINT xref_p47_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p47_not_deleted
    ADD CONSTRAINT xref_p47_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p47_not_deleted_old
    ADD CONSTRAINT xref_p47_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p47_not_deleted
    ADD CONSTRAINT xref_p47_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p47_not_deleted_old
    ADD CONSTRAINT xref_p47_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p48_deleted
    ADD CONSTRAINT xref_p48_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p48_deleted_old
    ADD CONSTRAINT xref_p48_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p48_deleted
    ADD CONSTRAINT xref_p48_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p48_deleted_old
    ADD CONSTRAINT xref_p48_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p48_deleted
    ADD CONSTRAINT xref_p48_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p48_deleted_old
    ADD CONSTRAINT xref_p48_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p48_deleted
    ADD CONSTRAINT xref_p48_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p48_deleted_old
    ADD CONSTRAINT xref_p48_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p48_not_deleted
    ADD CONSTRAINT xref_p48_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p48_not_deleted_old
    ADD CONSTRAINT xref_p48_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p48_not_deleted
    ADD CONSTRAINT xref_p48_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p48_not_deleted_old
    ADD CONSTRAINT xref_p48_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p48_not_deleted
    ADD CONSTRAINT xref_p48_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p48_not_deleted_old
    ADD CONSTRAINT xref_p48_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p48_not_deleted
    ADD CONSTRAINT xref_p48_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p48_not_deleted_old
    ADD CONSTRAINT xref_p48_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p49_deleted
    ADD CONSTRAINT xref_p49_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p49_deleted_old
    ADD CONSTRAINT xref_p49_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p49_deleted
    ADD CONSTRAINT xref_p49_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p49_deleted_old
    ADD CONSTRAINT xref_p49_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p49_deleted
    ADD CONSTRAINT xref_p49_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p49_deleted_old
    ADD CONSTRAINT xref_p49_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p49_deleted
    ADD CONSTRAINT xref_p49_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p49_deleted_old
    ADD CONSTRAINT xref_p49_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p49_not_deleted
    ADD CONSTRAINT xref_p49_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p49_not_deleted_old
    ADD CONSTRAINT xref_p49_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p49_not_deleted
    ADD CONSTRAINT xref_p49_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p49_not_deleted_old
    ADD CONSTRAINT xref_p49_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p49_not_deleted
    ADD CONSTRAINT xref_p49_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p49_not_deleted_old
    ADD CONSTRAINT xref_p49_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p49_not_deleted
    ADD CONSTRAINT xref_p49_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p49_not_deleted_old
    ADD CONSTRAINT xref_p49_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p4_deleted
    ADD CONSTRAINT xref_p4_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p4_deleted_old
    ADD CONSTRAINT xref_p4_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p4_deleted
    ADD CONSTRAINT xref_p4_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p4_deleted_old
    ADD CONSTRAINT xref_p4_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p4_deleted
    ADD CONSTRAINT xref_p4_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p4_deleted_old
    ADD CONSTRAINT xref_p4_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p4_deleted
    ADD CONSTRAINT xref_p4_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p4_deleted_old
    ADD CONSTRAINT xref_p4_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p4_not_deleted
    ADD CONSTRAINT xref_p4_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p4_not_deleted_old
    ADD CONSTRAINT xref_p4_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p4_not_deleted
    ADD CONSTRAINT xref_p4_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p4_not_deleted_old
    ADD CONSTRAINT xref_p4_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p4_not_deleted
    ADD CONSTRAINT xref_p4_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p4_not_deleted_old
    ADD CONSTRAINT xref_p4_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p4_not_deleted
    ADD CONSTRAINT xref_p4_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p4_not_deleted_old
    ADD CONSTRAINT xref_p4_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p50_deleted
    ADD CONSTRAINT xref_p50_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p50_deleted_old
    ADD CONSTRAINT xref_p50_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p50_deleted
    ADD CONSTRAINT xref_p50_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p50_deleted_old
    ADD CONSTRAINT xref_p50_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p50_deleted
    ADD CONSTRAINT xref_p50_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p50_deleted_old
    ADD CONSTRAINT xref_p50_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p50_deleted
    ADD CONSTRAINT xref_p50_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p50_deleted_old
    ADD CONSTRAINT xref_p50_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p50_not_deleted
    ADD CONSTRAINT xref_p50_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p50_not_deleted_old
    ADD CONSTRAINT xref_p50_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p50_not_deleted
    ADD CONSTRAINT xref_p50_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p50_not_deleted_old
    ADD CONSTRAINT xref_p50_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p50_not_deleted
    ADD CONSTRAINT xref_p50_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p50_not_deleted_old
    ADD CONSTRAINT xref_p50_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p50_not_deleted
    ADD CONSTRAINT xref_p50_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p50_not_deleted_old
    ADD CONSTRAINT xref_p50_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p51_deleted
    ADD CONSTRAINT xref_p51_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p51_deleted_old
    ADD CONSTRAINT xref_p51_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p51_deleted
    ADD CONSTRAINT xref_p51_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p51_deleted_old
    ADD CONSTRAINT xref_p51_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p51_deleted
    ADD CONSTRAINT xref_p51_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p51_deleted_old
    ADD CONSTRAINT xref_p51_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p51_deleted
    ADD CONSTRAINT xref_p51_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p51_deleted_old
    ADD CONSTRAINT xref_p51_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p51_not_deleted
    ADD CONSTRAINT xref_p51_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p51_not_deleted_old
    ADD CONSTRAINT xref_p51_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p51_not_deleted
    ADD CONSTRAINT xref_p51_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p51_not_deleted_old
    ADD CONSTRAINT xref_p51_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p51_not_deleted
    ADD CONSTRAINT xref_p51_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p51_not_deleted_old
    ADD CONSTRAINT xref_p51_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p51_not_deleted
    ADD CONSTRAINT xref_p51_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p51_not_deleted_old
    ADD CONSTRAINT xref_p51_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p52_deleted
    ADD CONSTRAINT xref_p52_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p52_deleted_old
    ADD CONSTRAINT xref_p52_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p52_deleted
    ADD CONSTRAINT xref_p52_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p52_deleted_old
    ADD CONSTRAINT xref_p52_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p52_deleted
    ADD CONSTRAINT xref_p52_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p52_deleted_old
    ADD CONSTRAINT xref_p52_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p52_deleted
    ADD CONSTRAINT xref_p52_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p52_deleted_old
    ADD CONSTRAINT xref_p52_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p52_not_deleted
    ADD CONSTRAINT xref_p52_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p52_not_deleted_old
    ADD CONSTRAINT xref_p52_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p52_not_deleted
    ADD CONSTRAINT xref_p52_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p52_not_deleted_old
    ADD CONSTRAINT xref_p52_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p52_not_deleted
    ADD CONSTRAINT xref_p52_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p52_not_deleted_old
    ADD CONSTRAINT xref_p52_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p52_not_deleted
    ADD CONSTRAINT xref_p52_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p52_not_deleted_old
    ADD CONSTRAINT xref_p52_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p53_deleted
    ADD CONSTRAINT xref_p53_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p53_deleted_old
    ADD CONSTRAINT xref_p53_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p53_deleted
    ADD CONSTRAINT xref_p53_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p53_deleted_old
    ADD CONSTRAINT xref_p53_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p53_deleted
    ADD CONSTRAINT xref_p53_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p53_deleted_old
    ADD CONSTRAINT xref_p53_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p53_deleted
    ADD CONSTRAINT xref_p53_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p53_deleted_old
    ADD CONSTRAINT xref_p53_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p53_not_deleted
    ADD CONSTRAINT xref_p53_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p53_not_deleted_old
    ADD CONSTRAINT xref_p53_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p53_not_deleted
    ADD CONSTRAINT xref_p53_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p53_not_deleted_old
    ADD CONSTRAINT xref_p53_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p53_not_deleted
    ADD CONSTRAINT xref_p53_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p53_not_deleted_old
    ADD CONSTRAINT xref_p53_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p53_not_deleted
    ADD CONSTRAINT xref_p53_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p53_not_deleted_old
    ADD CONSTRAINT xref_p53_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p54_deleted
    ADD CONSTRAINT xref_p54_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p54_deleted
    ADD CONSTRAINT xref_p54_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p54_deleted
    ADD CONSTRAINT xref_p54_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p54_deleted
    ADD CONSTRAINT xref_p54_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p54_not_deleted
    ADD CONSTRAINT xref_p54_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p54_not_deleted
    ADD CONSTRAINT xref_p54_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p54_not_deleted
    ADD CONSTRAINT xref_p54_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p54_not_deleted
    ADD CONSTRAINT xref_p54_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p55_deleted
    ADD CONSTRAINT xref_p55_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p55_deleted_old
    ADD CONSTRAINT xref_p55_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p55_deleted
    ADD CONSTRAINT xref_p55_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p55_deleted_old
    ADD CONSTRAINT xref_p55_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p55_deleted
    ADD CONSTRAINT xref_p55_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p55_deleted_old
    ADD CONSTRAINT xref_p55_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p55_deleted
    ADD CONSTRAINT xref_p55_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p55_deleted_old
    ADD CONSTRAINT xref_p55_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p55_not_deleted
    ADD CONSTRAINT xref_p55_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p55_not_deleted_old
    ADD CONSTRAINT xref_p55_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p55_not_deleted
    ADD CONSTRAINT xref_p55_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p55_not_deleted_old
    ADD CONSTRAINT xref_p55_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p55_not_deleted
    ADD CONSTRAINT xref_p55_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p55_not_deleted_old
    ADD CONSTRAINT xref_p55_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p55_not_deleted
    ADD CONSTRAINT xref_p55_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p55_not_deleted_old
    ADD CONSTRAINT xref_p55_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p5_deleted
    ADD CONSTRAINT xref_p5_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p5_deleted
    ADD CONSTRAINT xref_p5_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p5_deleted
    ADD CONSTRAINT xref_p5_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p5_deleted
    ADD CONSTRAINT xref_p5_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p5_not_deleted
    ADD CONSTRAINT xref_p5_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p5_not_deleted
    ADD CONSTRAINT xref_p5_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p5_not_deleted
    ADD CONSTRAINT xref_p5_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p5_not_deleted
    ADD CONSTRAINT xref_p5_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi) DEFERRABLE;


ALTER TABLE ONLY rnacen.xref_p6_deleted
    ADD CONSTRAINT xref_p6_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p6_deleted_old
    ADD CONSTRAINT xref_p6_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p6_deleted
    ADD CONSTRAINT xref_p6_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p6_deleted_old
    ADD CONSTRAINT xref_p6_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p6_deleted
    ADD CONSTRAINT xref_p6_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p6_deleted_old
    ADD CONSTRAINT xref_p6_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p6_deleted
    ADD CONSTRAINT xref_p6_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p6_deleted_old
    ADD CONSTRAINT xref_p6_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p6_not_deleted
    ADD CONSTRAINT xref_p6_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p6_not_deleted_old
    ADD CONSTRAINT xref_p6_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p6_not_deleted
    ADD CONSTRAINT xref_p6_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p6_not_deleted_old
    ADD CONSTRAINT xref_p6_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p6_not_deleted
    ADD CONSTRAINT xref_p6_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p6_not_deleted_old
    ADD CONSTRAINT xref_p6_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p6_not_deleted
    ADD CONSTRAINT xref_p6_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p6_not_deleted_old
    ADD CONSTRAINT xref_p6_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p7_deleted
    ADD CONSTRAINT xref_p7_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p7_deleted_old
    ADD CONSTRAINT xref_p7_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p7_deleted
    ADD CONSTRAINT xref_p7_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p7_deleted_old
    ADD CONSTRAINT xref_p7_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p7_deleted
    ADD CONSTRAINT xref_p7_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p7_deleted_old
    ADD CONSTRAINT xref_p7_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p7_deleted
    ADD CONSTRAINT xref_p7_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p7_deleted_old
    ADD CONSTRAINT xref_p7_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p7_not_deleted
    ADD CONSTRAINT xref_p7_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p7_not_deleted_old
    ADD CONSTRAINT xref_p7_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p7_not_deleted
    ADD CONSTRAINT xref_p7_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p7_not_deleted_old
    ADD CONSTRAINT xref_p7_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p7_not_deleted
    ADD CONSTRAINT xref_p7_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p7_not_deleted_old
    ADD CONSTRAINT xref_p7_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p7_not_deleted
    ADD CONSTRAINT xref_p7_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p7_not_deleted_old
    ADD CONSTRAINT xref_p7_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p8_deleted
    ADD CONSTRAINT xref_p8_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p8_deleted_old
    ADD CONSTRAINT xref_p8_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p8_deleted
    ADD CONSTRAINT xref_p8_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p8_deleted_old
    ADD CONSTRAINT xref_p8_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p8_deleted
    ADD CONSTRAINT xref_p8_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p8_deleted_old
    ADD CONSTRAINT xref_p8_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p8_deleted
    ADD CONSTRAINT xref_p8_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p8_deleted_old
    ADD CONSTRAINT xref_p8_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p8_not_deleted
    ADD CONSTRAINT xref_p8_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p8_not_deleted_old
    ADD CONSTRAINT xref_p8_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p8_not_deleted
    ADD CONSTRAINT xref_p8_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p8_not_deleted_old
    ADD CONSTRAINT xref_p8_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p8_not_deleted
    ADD CONSTRAINT xref_p8_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p8_not_deleted_old
    ADD CONSTRAINT xref_p8_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p8_not_deleted
    ADD CONSTRAINT xref_p8_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p8_not_deleted_old
    ADD CONSTRAINT xref_p8_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p9_deleted
    ADD CONSTRAINT xref_p9_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p9_deleted_old
    ADD CONSTRAINT xref_p9_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p9_deleted
    ADD CONSTRAINT xref_p9_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p9_deleted_old
    ADD CONSTRAINT xref_p9_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p9_deleted
    ADD CONSTRAINT xref_p9_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p9_deleted_old
    ADD CONSTRAINT xref_p9_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p9_deleted
    ADD CONSTRAINT xref_p9_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p9_deleted_old
    ADD CONSTRAINT xref_p9_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p9_not_deleted
    ADD CONSTRAINT xref_p9_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p9_not_deleted_old
    ADD CONSTRAINT xref_p9_not_deleted_fk1_old FOREIGN KEY (created) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p9_not_deleted
    ADD CONSTRAINT xref_p9_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p9_not_deleted_old
    ADD CONSTRAINT xref_p9_not_deleted_fk2_old FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database(id);


ALTER TABLE ONLY rnacen.xref_p9_not_deleted
    ADD CONSTRAINT xref_p9_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p9_not_deleted_old
    ADD CONSTRAINT xref_p9_not_deleted_fk3_old FOREIGN KEY (last) REFERENCES rnacen.rnc_release(id);


ALTER TABLE ONLY rnacen.xref_p9_not_deleted
    ADD CONSTRAINT xref_p9_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);


ALTER TABLE ONLY rnacen.xref_p9_not_deleted_old
    ADD CONSTRAINT xref_p9_not_deleted_fk4_old FOREIGN KEY (upi) REFERENCES rnacen.rna(upi);



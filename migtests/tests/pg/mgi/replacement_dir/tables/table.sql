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


CREATE TABLE mgd.acc_accession (
    _accession_key integer NOT NULL,
    accid text NOT NULL,
    prefixpart text,
    numericpart integer,
    _logicaldb_key integer NOT NULL,
    _object_key integer NOT NULL,
    _mgitype_key integer NOT NULL,
    private smallint NOT NULL,
    preferred smallint NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.acc_accessionmax (
    prefixpart text NOT NULL,
    maxnumericpart integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.acc_accessionreference (
    _accession_key integer NOT NULL,
    _refs_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.acc_actualdb (
    _actualdb_key integer DEFAULT nextval('mgd.acc_actualdb_seq'::regclass) NOT NULL,
    _logicaldb_key integer NOT NULL,
    name text NOT NULL,
    active smallint NOT NULL,
    url text NOT NULL,
    allowsmultiple smallint NOT NULL,
    delimiter character(8),
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.acc_logicaldb (
    _logicaldb_key integer DEFAULT nextval('mgd.acc_logicaldb_seq'::regclass) NOT NULL,
    name text NOT NULL,
    description text,
    _organism_key integer,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.acc_mgitype (
    _mgitype_key integer NOT NULL,
    name text NOT NULL,
    tablename text NOT NULL,
    primarykeyname text NOT NULL,
    identitycolumnname text,
    dbview text,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.all_allele (
    _allele_key integer DEFAULT nextval('mgd.all_allele_seq'::regclass) NOT NULL,
    _marker_key integer,
    _strain_key integer NOT NULL,
    _mode_key integer NOT NULL,
    _allele_type_key integer NOT NULL,
    _allele_status_key integer NOT NULL,
    _transmission_key integer NOT NULL,
    _collection_key integer NOT NULL,
    symbol text NOT NULL,
    name text NOT NULL,
    iswildtype smallint NOT NULL,
    isextinct smallint NOT NULL,
    ismixed smallint NOT NULL,
    _refs_key integer,
    _markerallele_status_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    _approvedby_key integer,
    approval_date timestamp without time zone DEFAULT now(),
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.all_allele_cellline (
    _assoc_key integer DEFAULT nextval('mgd.all_allele_cellline_seq'::regclass) NOT NULL,
    _allele_key integer NOT NULL,
    _mutantcellline_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.all_cellline (
    _cellline_key integer DEFAULT nextval('mgd.all_cellline_seq'::regclass) NOT NULL,
    cellline text NOT NULL,
    _cellline_type_key integer NOT NULL,
    _strain_key integer NOT NULL,
    _derivation_key integer,
    ismutant smallint NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.all_cellline_derivation (
    _derivation_key integer DEFAULT nextval('mgd.all_cellline_derivation_seq'::regclass) NOT NULL,
    name text,
    description text,
    _vector_key integer NOT NULL,
    _vectortype_key integer NOT NULL,
    _parentcellline_key integer NOT NULL,
    _derivationtype_key integer NOT NULL,
    _creator_key integer NOT NULL,
    _refs_key integer,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mgi_user (
    _user_key integer NOT NULL,
    _usertype_key integer NOT NULL,
    _userstatus_key integer NOT NULL,
    login text NOT NULL,
    name text NOT NULL,
    orcid text,
    _group_key integer,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.prb_strain (
    _strain_key integer DEFAULT nextval('mgd.prb_strain_seq'::regclass) NOT NULL,
    _species_key integer NOT NULL,
    _straintype_key integer NOT NULL,
    strain text NOT NULL,
    standard smallint NOT NULL,
    private smallint NOT NULL,
    geneticbackground smallint NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.voc_term (
    _term_key integer DEFAULT nextval('mgd.voc_term_seq'::regclass) NOT NULL,
    _vocab_key integer NOT NULL,
    term text,
    abbreviation text,
    note text,
    sequencenum integer,
    isobsolete smallint,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mgi_organism (
    _organism_key integer DEFAULT nextval('mgd.mgi_organism_seq'::regclass) NOT NULL,
    commonname text NOT NULL,
    latinname text,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mgi_relationship (
    _relationship_key integer DEFAULT nextval('mgd.mgi_relationship_seq'::regclass) NOT NULL,
    _category_key integer NOT NULL,
    _object_key_1 integer NOT NULL,
    _object_key_2 integer NOT NULL,
    _relationshipterm_key integer NOT NULL,
    _qualifier_key integer NOT NULL,
    _evidence_key integer NOT NULL,
    _refs_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mrk_marker (
    _marker_key integer DEFAULT nextval('mgd.mrk_marker_seq'::regclass) NOT NULL,
    _organism_key integer NOT NULL,
    _marker_status_key integer NOT NULL,
    _marker_type_key integer NOT NULL,
    symbol text NOT NULL,
    name text NOT NULL,
    chromosome text NOT NULL,
    cytogeneticoffset text,
    cmoffset double precision,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.all_allele_mutation (
    _assoc_key integer DEFAULT nextval('mgd.all_allele_mutation_seq'::regclass) NOT NULL,
    _allele_key integer NOT NULL,
    _mutation_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.voc_annot (
    _annot_key integer DEFAULT nextval('mgd.voc_annot_seq'::regclass) NOT NULL,
    _annottype_key integer NOT NULL,
    _object_key integer NOT NULL,
    _term_key integer NOT NULL,
    _qualifier_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.bib_citation_cache (
    _refs_key integer NOT NULL,
    numericpart integer,
    jnumid text,
    mgiid text NOT NULL,
    pubmedid text,
    doiid text,
    journal text,
    citation text NOT NULL,
    short_citation text NOT NULL,
    referencetype text NOT NULL,
    _relevance_key integer NOT NULL,
    relevanceterm text NOT NULL,
    isreviewarticle smallint NOT NULL,
    isreviewarticlestring character(3) NOT NULL
);


CREATE TABLE mgd.gxd_allelepair (
    _allelepair_key integer DEFAULT nextval('mgd.gxd_allelepair_seq'::regclass) NOT NULL,
    _genotype_key integer NOT NULL,
    _allele_key_1 integer NOT NULL,
    _allele_key_2 integer,
    _marker_key integer,
    _mutantcellline_key_1 integer,
    _mutantcellline_key_2 integer,
    _pairstate_key integer NOT NULL,
    _compound_key integer NOT NULL,
    sequencenum integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.voc_annottype (
    _annottype_key integer NOT NULL,
    _mgitype_key integer NOT NULL,
    _vocab_key integer NOT NULL,
    _evidencevocab_key integer NOT NULL,
    _qualifiervocab_key integer NOT NULL,
    name text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.all_cre_cache (
    _cache_key integer NOT NULL,
    _allele_key integer NOT NULL,
    _allele_type_key integer NOT NULL,
    _emapa_term_key integer,
    _celltype_term_key integer,
    _stage_key integer,
    _assay_key integer,
    strength text,
    accid text NOT NULL,
    symbol text NOT NULL,
    name text NOT NULL,
    alleletype text NOT NULL,
    drivernote text NOT NULL,
    emapaterm text,
    age text,
    agemin numeric,
    agemax numeric,
    expressed integer,
    hasimage integer,
    cresystemlabel text,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.all_knockout_cache (
    _knockout_key integer NOT NULL,
    _marker_key integer NOT NULL,
    _allele_key integer,
    holder text NOT NULL,
    repository text NOT NULL,
    companyid text,
    nihid text,
    jrsid text,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.all_label (
    _allele_key integer NOT NULL,
    _label_status_key integer NOT NULL,
    priority integer NOT NULL,
    label text NOT NULL,
    labeltype text NOT NULL,
    labeltypename text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_allelegenotype (
    _genotype_key integer NOT NULL,
    _marker_key integer,
    _allele_key integer NOT NULL,
    sequencenum integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mgi_reference_assoc (
    _assoc_key integer DEFAULT nextval('mgd.mgi_reference_assoc_seq'::regclass) NOT NULL,
    _refs_key integer NOT NULL,
    _object_key integer NOT NULL,
    _mgitype_key integer NOT NULL,
    _refassoctype_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.all_variant (
    _variant_key integer DEFAULT nextval('mgd.all_variant_seq'::regclass) NOT NULL,
    _allele_key integer NOT NULL,
    _sourcevariant_key integer,
    _strain_key integer NOT NULL,
    isreviewed smallint NOT NULL,
    description text,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.all_variant_sequence (
    _variantsequence_key integer DEFAULT nextval('mgd.all_variantsequence_seq'::regclass) NOT NULL,
    _variant_key integer NOT NULL,
    _sequence_type_key integer NOT NULL,
    startcoordinate numeric,
    endcoordinate numeric,
    referencesequence text,
    variantsequence text,
    version text,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.bib_refs (
    _refs_key integer DEFAULT nextval('mgd.bib_refs_seq'::regclass) NOT NULL,
    _referencetype_key integer NOT NULL,
    authors text,
    _primary text,
    title text,
    journal text,
    vol text,
    issue text,
    date text,
    year integer,
    pgs text,
    abstract text,
    isreviewarticle smallint NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_expression (
    _expression_key integer NOT NULL,
    _assay_key integer NOT NULL,
    _refs_key integer NOT NULL,
    _assaytype_key integer NOT NULL,
    _genotype_key integer NOT NULL,
    _marker_key integer NOT NULL,
    _emapa_term_key integer NOT NULL,
    _celltype_term_key integer,
    _stage_key integer NOT NULL,
    _specimen_key integer,
    _gellane_key integer,
    resultnote text,
    expressed smallint NOT NULL,
    strength text,
    age text NOT NULL,
    agemin numeric,
    agemax numeric,
    isrecombinase smallint NOT NULL,
    isforgxd smallint NOT NULL,
    hasimage smallint NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_index (
    _index_key integer DEFAULT nextval('mgd.gxd_index_seq'::regclass) NOT NULL,
    _refs_key integer NOT NULL,
    _marker_key integer NOT NULL,
    _priority_key integer NOT NULL,
    _conditionalmutants_key integer NOT NULL,
    comments text,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.img_image (
    _image_key integer DEFAULT nextval('mgd.img_image_seq'::regclass) NOT NULL,
    _imageclass_key integer NOT NULL,
    _imagetype_key integer NOT NULL,
    _refs_key integer NOT NULL,
    _thumbnailimage_key integer,
    xdim integer,
    ydim integer,
    figurelabel text NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mgi_refassoctype (
    _refassoctype_key integer NOT NULL,
    _mgitype_key integer,
    assoctype text NOT NULL,
    allowonlyone smallint NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mgi_synonym (
    _synonym_key integer DEFAULT nextval('mgd.mgi_synonym_seq'::regclass) NOT NULL,
    _object_key integer NOT NULL,
    _mgitype_key integer NOT NULL,
    _synonymtype_key integer NOT NULL,
    _refs_key integer,
    synonym text NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mgi_synonymtype (
    _synonymtype_key integer NOT NULL,
    _mgitype_key integer,
    _organism_key integer,
    synonymtype text NOT NULL,
    definition text,
    allowonlyone smallint NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mld_expts (
    _expt_key integer DEFAULT nextval('mgd.mld_expts_seq'::regclass) NOT NULL,
    _refs_key integer NOT NULL,
    expttype text NOT NULL,
    tag integer NOT NULL,
    chromosome text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mld_notes (
    _refs_key integer NOT NULL,
    note text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mrk_do_cache (
    _cache_key integer NOT NULL,
    _organism_key integer NOT NULL,
    _marker_key integer NOT NULL,
    _genotype_key integer NOT NULL,
    _term_key integer NOT NULL,
    _refs_key integer NOT NULL,
    omimcategory3 integer NOT NULL,
    qualifier text,
    term text NOT NULL,
    termid text NOT NULL,
    jnumid text NOT NULL,
    header text NOT NULL,
    headerfootnote text,
    genotypefootnote text,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mrk_reference (
    _marker_key integer NOT NULL,
    _refs_key integer NOT NULL,
    mgiid text NOT NULL,
    jnumid text NOT NULL,
    pubmedid text,
    jnum integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mrk_strainmarker (
    _strainmarker_key integer DEFAULT nextval('mgd.mrk_strainmarker_seq'::regclass) NOT NULL,
    _strain_key integer NOT NULL,
    _marker_key integer,
    _refs_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.prb_reference (
    _reference_key integer DEFAULT nextval('mgd.prb_reference_seq'::regclass) NOT NULL,
    _probe_key integer NOT NULL,
    _refs_key integer NOT NULL,
    hasrmap smallint NOT NULL,
    hassequence smallint NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.prb_source (
    _source_key integer DEFAULT nextval('mgd.prb_source_seq'::regclass) NOT NULL,
    _segmenttype_key integer NOT NULL,
    _vector_key integer NOT NULL,
    _organism_key integer NOT NULL,
    _strain_key integer NOT NULL,
    _tissue_key integer NOT NULL,
    _gender_key integer NOT NULL,
    _cellline_key integer NOT NULL,
    _refs_key integer,
    name text,
    description text,
    age text NOT NULL,
    agemin numeric NOT NULL,
    agemax numeric NOT NULL,
    iscuratoredited smallint NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.voc_evidence (
    _annotevidence_key integer DEFAULT nextval('mgd.voc_evidence_seq'::regclass) NOT NULL,
    _annot_key integer NOT NULL,
    _evidenceterm_key integer NOT NULL,
    _refs_key integer NOT NULL,
    inferredfrom text,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.bib_books (
    _refs_key integer NOT NULL,
    book_au text,
    book_title text,
    place text,
    publisher text,
    series_ed text,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.bib_workflow_status (
    _assoc_key integer DEFAULT nextval('mgd.bib_workflow_status_seq'::regclass) NOT NULL,
    _refs_key integer NOT NULL,
    _group_key integer NOT NULL,
    _status_key integer NOT NULL,
    iscurrent smallint DEFAULT 1 NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.bib_notes (
    _refs_key integer NOT NULL,
    note text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_assay (
    _assay_key integer DEFAULT nextval('mgd.gxd_assay_seq'::regclass) NOT NULL,
    _assaytype_key integer NOT NULL,
    _refs_key integer NOT NULL,
    _marker_key integer NOT NULL,
    _probeprep_key integer,
    _antibodyprep_key integer,
    _imagepane_key integer,
    _reportergene_key integer,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_specimen (
    _specimen_key integer DEFAULT nextval('mgd.gxd_specimen_seq'::regclass) NOT NULL,
    _assay_key integer NOT NULL,
    _embedding_key integer NOT NULL,
    _fixation_key integer NOT NULL,
    _genotype_key integer NOT NULL,
    sequencenum integer NOT NULL,
    specimenlabel text,
    sex text NOT NULL,
    age text NOT NULL,
    agemin numeric,
    agemax numeric,
    agenote text,
    hybridization text NOT NULL,
    specimennote text,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.bib_workflow_data (
    _assoc_key integer DEFAULT nextval('mgd.bib_workflow_data_seq'::regclass) NOT NULL,
    _refs_key integer NOT NULL,
    haspdf smallint DEFAULT 0 NOT NULL,
    _supplemental_key integer NOT NULL,
    linksupplemental text,
    _extractedtext_key integer NOT NULL,
    extractedtext text,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.bib_workflow_relevance (
    _assoc_key integer DEFAULT nextval('mgd.bib_workflow_relevance_seq'::regclass) NOT NULL,
    _refs_key integer NOT NULL,
    _relevance_key integer NOT NULL,
    iscurrent smallint DEFAULT 1 NOT NULL,
    confidence double precision,
    version text,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.bib_workflow_tag (
    _assoc_key integer DEFAULT nextval('mgd.bib_workflow_tag_seq'::regclass) NOT NULL,
    _refs_key integer NOT NULL,
    _tag_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.crs_cross (
    _cross_key integer NOT NULL,
    type text NOT NULL,
    _femalestrain_key integer NOT NULL,
    femaleallele1 character(1),
    femaleallele2 character(1),
    _malestrain_key integer NOT NULL,
    maleallele1 character(1),
    maleallele2 character(1),
    abbrevho text,
    _strainho_key integer NOT NULL,
    abbrevht text,
    _strainht_key integer NOT NULL,
    whosecross text,
    allelefromsegparent smallint NOT NULL,
    f1directionknown smallint NOT NULL,
    nprogeny integer,
    displayed smallint NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.crs_matrix (
    _cross_key integer NOT NULL,
    _marker_key integer,
    othersymbol text,
    chromosome text NOT NULL,
    rownumber integer NOT NULL,
    notes text,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.crs_progeny (
    _cross_key integer NOT NULL,
    sequencenum integer NOT NULL,
    name text,
    sex character(1) NOT NULL,
    notes text,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.crs_references (
    _cross_key integer NOT NULL,
    _marker_key integer NOT NULL,
    _refs_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.crs_typings (
    _cross_key integer NOT NULL,
    rownumber integer NOT NULL,
    colnumber integer NOT NULL,
    data text,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.dag_closure (
    _dag_key integer NOT NULL,
    _mgitype_key integer NOT NULL,
    _ancestor_key integer NOT NULL,
    _descendent_key integer NOT NULL,
    _ancestorobject_key integer NOT NULL,
    _descendentobject_key integer NOT NULL,
    _ancestorlabel_key integer NOT NULL,
    _descendentlabel_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.dag_dag (
    _dag_key integer NOT NULL,
    _refs_key integer NOT NULL,
    _mgitype_key integer NOT NULL,
    name text NOT NULL,
    abbreviation character(5) NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.dag_edge (
    _edge_key integer NOT NULL,
    _dag_key integer NOT NULL,
    _parent_key integer NOT NULL,
    _child_key integer NOT NULL,
    _label_key integer NOT NULL,
    sequencenum integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.dag_label (
    _label_key integer NOT NULL,
    label text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.dag_node (
    _node_key integer NOT NULL,
    _dag_key integer NOT NULL,
    _object_key integer NOT NULL,
    _label_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.voc_vocabdag (
    _vocab_key integer NOT NULL,
    _dag_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.go_tracking (
    _marker_key integer NOT NULL,
    isreferencegene smallint NOT NULL,
    _completedby_key integer,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    completion_date timestamp without time zone DEFAULT now(),
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_antibody (
    _antibody_key integer DEFAULT nextval('mgd.gxd_antibody_seq'::regclass) NOT NULL,
    _antibodyclass_key integer NOT NULL,
    _antibodytype_key integer NOT NULL,
    _organism_key integer NOT NULL,
    _antigen_key integer NOT NULL,
    antibodyname text NOT NULL,
    antibodynote text,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_antigen (
    _antigen_key integer DEFAULT nextval('mgd.gxd_antigen_seq'::regclass) NOT NULL,
    _source_key integer NOT NULL,
    antigenname text NOT NULL,
    regioncovered text,
    antigennote text,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_antibodyalias (
    _antibodyalias_key integer DEFAULT nextval('mgd.gxd_antibodyalias_seq'::regclass) NOT NULL,
    _antibody_key integer NOT NULL,
    _refs_key integer,
    alias text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_antibodymarker (
    _antibodymarker_key integer DEFAULT nextval('mgd.gxd_antibodymarker_seq'::regclass) NOT NULL,
    _antibody_key integer NOT NULL,
    _marker_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_antibodyprep (
    _antibodyprep_key integer DEFAULT nextval('mgd.gxd_antibodyprep_seq'::regclass) NOT NULL,
    _antibody_key integer NOT NULL,
    _secondary_key integer NOT NULL,
    _label_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_gellane (
    _gellane_key integer DEFAULT nextval('mgd.gxd_gellane_seq'::regclass) NOT NULL,
    _assay_key integer NOT NULL,
    _genotype_key integer NOT NULL,
    _gelrnatype_key integer NOT NULL,
    _gelcontrol_key integer NOT NULL,
    sequencenum integer NOT NULL,
    lanelabel text,
    sampleamount text,
    sex text NOT NULL,
    age text NOT NULL,
    agemin numeric,
    agemax numeric,
    agenote text,
    lanenote text,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_insituresult (
    _result_key integer DEFAULT nextval('mgd.gxd_insituresult_seq'::regclass) NOT NULL,
    _specimen_key integer NOT NULL,
    _strength_key integer NOT NULL,
    _pattern_key integer NOT NULL,
    sequencenum integer NOT NULL,
    resultnote text,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_insituresultimage (
    _resultimage_key integer DEFAULT nextval('mgd.gxd_insituresultimage_seq'::regclass) NOT NULL,
    _result_key integer NOT NULL,
    _imagepane_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_assaytype (
    _assaytype_key integer DEFAULT nextval('mgd.gxd_assaytype_seq'::regclass) NOT NULL,
    assaytype text NOT NULL,
    isrnaassay smallint NOT NULL,
    isgelassay smallint NOT NULL,
    sequencenum integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_assaynote (
    _assaynote_key integer DEFAULT nextval('mgd.gxd_assaynote_seq'::regclass) NOT NULL,
    _assay_key integer NOT NULL,
    assaynote text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_gelband (
    _gelband_key integer DEFAULT nextval('mgd.gxd_gelband_seq'::regclass) NOT NULL,
    _gellane_key integer NOT NULL,
    _gelrow_key integer NOT NULL,
    _strength_key integer NOT NULL,
    bandnote text,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_gelrow (
    _gelrow_key integer DEFAULT nextval('mgd.gxd_gelrow_seq'::regclass) NOT NULL,
    _assay_key integer NOT NULL,
    _gelunits_key integer NOT NULL,
    sequencenum integer NOT NULL,
    size numeric,
    rownote text,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_genotype (
    _genotype_key integer DEFAULT nextval('mgd.gxd_genotype_seq'::regclass) NOT NULL,
    _strain_key integer NOT NULL,
    isconditional smallint NOT NULL,
    note text,
    _existsas_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_gellanestructure (
    _gellanestructure_key integer DEFAULT nextval('mgd.gxd_gellanestructure_seq'::regclass) NOT NULL,
    _gellane_key integer NOT NULL,
    _emapa_term_key integer NOT NULL,
    _stage_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_theilerstage (
    _stage_key integer NOT NULL,
    stage integer NOT NULL,
    description text,
    dpcmin numeric NOT NULL,
    dpcmax numeric NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.voc_annotheader (
    _annotheader_key integer DEFAULT nextval('mgd.voc_annotheader_seq'::regclass) NOT NULL,
    _annottype_key integer NOT NULL,
    _object_key integer NOT NULL,
    _term_key integer NOT NULL,
    sequencenum integer NOT NULL,
    isnormal smallint NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    _approvedby_key integer,
    approval_date timestamp without time zone DEFAULT now(),
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_htexperiment (
    _experiment_key integer DEFAULT nextval('mgd.gxd_htexperiment_seq'::regclass) NOT NULL,
    _source_key integer NOT NULL,
    name text,
    description text,
    release_date timestamp without time zone,
    lastupdate_date timestamp without time zone,
    evaluated_date timestamp without time zone,
    _evaluationstate_key integer NOT NULL,
    _curationstate_key integer NOT NULL,
    _studytype_key integer NOT NULL,
    _experimenttype_key integer NOT NULL,
    _evaluatedby_key integer,
    _initialcuratedby_key integer,
    _lastcuratedby_key integer,
    initial_curated_date timestamp without time zone,
    last_curated_date timestamp without time zone,
    confidence numeric DEFAULT 0.0 NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_htexperimentvariable (
    _experimentvariable_key integer DEFAULT nextval('mgd.gxd_htexperimentvariable_seq'::regclass) NOT NULL,
    _experiment_key integer NOT NULL,
    _term_key integer NOT NULL
);


CREATE TABLE mgd.gxd_htrawsample (
    _rawsample_key integer DEFAULT nextval('mgd.gxd_htrawsample_seq'::regclass) NOT NULL,
    _experiment_key integer NOT NULL,
    accid text,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_htsample (
    _sample_key integer DEFAULT nextval('mgd.gxd_htsample_seq'::regclass) NOT NULL,
    _experiment_key integer NOT NULL,
    _relevance_key integer NOT NULL,
    name text,
    age text,
    agemin numeric,
    agemax numeric,
    _organism_key integer NOT NULL,
    _sex_key integer NOT NULL,
    _emapa_key integer,
    _stage_key integer,
    _genotype_key integer NOT NULL,
    _celltype_term_key integer,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_htsample_rnaseq (
    _rnaseq_key integer DEFAULT nextval('mgd.gxd_htsample_rnaseq_seq'::regclass) NOT NULL,
    _sample_key integer NOT NULL,
    _rnaseqcombined_key integer NOT NULL,
    _marker_key integer NOT NULL,
    averagetpm numeric NOT NULL,
    quantilenormalizedtpm numeric NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_htsample_rnaseqcombined (
    _rnaseqcombined_key integer DEFAULT nextval('mgd.gxd_htsample_rnaseqcombined_seq'::regclass) NOT NULL,
    _marker_key integer NOT NULL,
    _level_key integer NOT NULL,
    numberofbiologicalreplicates integer NOT NULL,
    averagequantilenormalizedtpm numeric NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_htsample_rnaseqset (
    _rnaseqset_key integer DEFAULT nextval('mgd.gxd_htsample_rnaseqset_seq'::regclass) NOT NULL,
    _experiment_key integer NOT NULL,
    age text NOT NULL,
    _organism_key integer NOT NULL,
    _sex_key integer NOT NULL,
    _emapa_key integer NOT NULL,
    _stage_key integer NOT NULL,
    _genotype_key integer NOT NULL,
    note text,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_htsample_rnaseqset_cache (
    _assoc_key integer DEFAULT nextval('mgd.gxd_htsample_rnaseqset_cache_seq'::regclass) NOT NULL,
    _rnaseqcombined_key integer NOT NULL,
    _rnaseqset_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_htsample_rnaseqsetmember (
    _rnaseqsetmember_key integer DEFAULT nextval('mgd.gxd_htsample_rnaseqsetmember_seq'::regclass) NOT NULL,
    _rnaseqset_key integer NOT NULL,
    _sample_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_index_stages (
    _indexstage_key integer DEFAULT nextval('mgd.gxd_indexstage_seq'::regclass) NOT NULL,
    _index_key integer NOT NULL,
    _indexassay_key integer NOT NULL,
    _stageid_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_isresultcelltype (
    _resultcelltype_key integer DEFAULT nextval('mgd.gxd_isresultcelltype_seq'::regclass) NOT NULL,
    _result_key integer NOT NULL,
    _celltype_term_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.img_imagepane (
    _imagepane_key integer DEFAULT nextval('mgd.img_imagepane_seq'::regclass) NOT NULL,
    _image_key integer NOT NULL,
    panelabel text,
    x integer,
    y integer,
    width integer,
    height integer,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_isresultstructure (
    _resultstructure_key integer DEFAULT nextval('mgd.gxd_isresultstructure_seq'::regclass) NOT NULL,
    _result_key integer NOT NULL,
    _emapa_term_key integer NOT NULL,
    _stage_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.gxd_probeprep (
    _probeprep_key integer DEFAULT nextval('mgd.gxd_probeprep_seq'::regclass) NOT NULL,
    _probe_key integer NOT NULL,
    _sense_key integer NOT NULL,
    _label_key integer NOT NULL,
    _visualization_key integer NOT NULL,
    type text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.prb_probe (
    _probe_key integer DEFAULT nextval('mgd.prb_probe_seq'::regclass) NOT NULL,
    name text NOT NULL,
    derivedfrom integer,
    ampprimer integer,
    _source_key integer NOT NULL,
    _vector_key integer NOT NULL,
    _segmenttype_key integer NOT NULL,
    primer1sequence text,
    primer2sequence text,
    regioncovered text,
    insertsite text,
    insertsize text,
    productsize text,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.img_imagepane_assoc (
    _assoc_key integer DEFAULT nextval('mgd.img_imagepane_assoc_seq'::regclass) NOT NULL,
    _imagepane_key integer NOT NULL,
    _mgitype_key integer NOT NULL,
    _object_key integer NOT NULL,
    isprimary smallint NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mgi_note (
    _note_key integer DEFAULT nextval('mgd.mgi_note_seq'::regclass) NOT NULL,
    _object_key integer NOT NULL,
    _mgitype_key integer NOT NULL,
    _notetype_key integer NOT NULL,
    note text NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.map_coord_collection (
    _collection_key integer NOT NULL,
    name text NOT NULL,
    abbreviation text NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.map_coord_feature (
    _feature_key integer NOT NULL,
    _map_key integer NOT NULL,
    _mgitype_key integer NOT NULL,
    _object_key integer NOT NULL,
    startcoordinate numeric NOT NULL,
    endcoordinate numeric NOT NULL,
    strand character(1),
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.map_coordinate (
    _map_key integer NOT NULL,
    _collection_key integer NOT NULL,
    _object_key integer,
    _mgitype_key integer,
    _maptype_key integer NOT NULL,
    _units_key integer NOT NULL,
    length integer NOT NULL,
    sequencenum integer NOT NULL,
    name text,
    abbreviation text,
    version text,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mrk_chromosome (
    _chromosome_key integer DEFAULT nextval('mgd.mrk_chromosome_seq'::regclass) NOT NULL,
    _organism_key integer NOT NULL,
    chromosome text NOT NULL,
    sequencenum integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mgi_dbinfo (
    public_version text NOT NULL,
    product_name text NOT NULL,
    schema_version text NOT NULL,
    snp_schema_version text NOT NULL,
    snp_data_version text NOT NULL,
    lastdump_date timestamp without time zone DEFAULT now() NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mgi_keyvalue (
    _keyvalue_key integer DEFAULT nextval('mgd.mgi_keyvalue_seq'::regclass) NOT NULL,
    _object_key integer NOT NULL,
    _mgitype_key integer NOT NULL,
    key text NOT NULL,
    value text NOT NULL,
    sequencenum integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mgi_notetype (
    _notetype_key integer NOT NULL,
    _mgitype_key integer,
    notetype text NOT NULL,
    private smallint NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mgi_organism_mgitype (
    _assoc_key integer DEFAULT nextval('mgd.mgi_organism_mgitype_seq'::regclass) NOT NULL,
    _organism_key integer NOT NULL,
    _mgitype_key integer NOT NULL,
    sequencenum integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mgi_property (
    _property_key integer DEFAULT nextval('mgd.mgi_property_seq'::regclass) NOT NULL,
    _propertytype_key integer NOT NULL,
    _propertyterm_key integer NOT NULL,
    _object_key integer NOT NULL,
    _mgitype_key integer NOT NULL,
    value text NOT NULL,
    sequencenum integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mgi_propertytype (
    _propertytype_key integer NOT NULL,
    _mgitype_key integer NOT NULL,
    _vocab_key integer,
    propertytype text NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mgi_relationship_category (
    _category_key integer NOT NULL,
    name text NOT NULL,
    _relationshipvocab_key integer NOT NULL,
    _relationshipdag_key integer,
    _mgitype_key_1 integer NOT NULL,
    _mgitype_key_2 integer NOT NULL,
    _qualifiervocab_key integer NOT NULL,
    _evidencevocab_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mgi_relationship_property (
    _relationshipproperty_key integer DEFAULT nextval('mgd.mgi_relationship_property_seq'::regclass) NOT NULL,
    _relationship_key integer NOT NULL,
    _propertyname_key integer NOT NULL,
    value text NOT NULL,
    sequencenum integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mgi_set (
    _set_key integer NOT NULL,
    _mgitype_key integer NOT NULL,
    name text NOT NULL,
    sequencenum integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mgi_setmember (
    _setmember_key integer NOT NULL,
    _set_key integer NOT NULL,
    _object_key integer NOT NULL,
    label text,
    sequencenum integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mgi_setmember_emapa (
    _setmember_emapa_key integer NOT NULL,
    _setmember_key integer NOT NULL,
    _stage_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mgi_translation (
    _translation_key integer DEFAULT nextval('mgd.mgi_translation_seq'::regclass) NOT NULL,
    _translationtype_key integer NOT NULL,
    _object_key integer NOT NULL,
    badname text NOT NULL,
    sequencenum integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mgi_translationtype (
    _translationtype_key integer NOT NULL,
    _mgitype_key integer NOT NULL,
    _vocab_key integer,
    translationtype text NOT NULL,
    compressionchars text,
    regularexpression smallint NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.voc_vocab (
    _vocab_key integer NOT NULL,
    _refs_key integer NOT NULL,
    _logicaldb_key integer NOT NULL,
    issimple smallint,
    isprivate smallint,
    name text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mld_assay_types (
    _assay_type_key integer DEFAULT nextval('mgd.mld_assay_types_seq'::regclass) NOT NULL,
    description text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mld_concordance (
    _expt_key integer NOT NULL,
    sequencenum integer NOT NULL,
    _marker_key integer,
    chromosome text,
    cpp integer NOT NULL,
    cpn integer NOT NULL,
    cnp integer NOT NULL,
    cnn integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mld_contig (
    _contig_key integer NOT NULL,
    _expt_key integer NOT NULL,
    mincm numeric,
    maxcm numeric,
    name text,
    minlink integer,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mld_contigprobe (
    _contig_key integer NOT NULL,
    sequencenum integer NOT NULL,
    _probe_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mld_expt_marker (
    _assoc_key integer DEFAULT nextval('mgd.mld_expt_marker_seq'::regclass) NOT NULL,
    _expt_key integer NOT NULL,
    _marker_key integer NOT NULL,
    _allele_key integer,
    _assay_type_key integer NOT NULL,
    sequencenum integer NOT NULL,
    description text,
    matrixdata smallint NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mld_expt_notes (
    _expt_key integer NOT NULL,
    note text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mld_fish (
    _expt_key integer NOT NULL,
    band text,
    _strain_key integer NOT NULL,
    cellorigin text,
    karyotype text,
    robertsonians text,
    label text,
    nummetaphase integer,
    totalsingle integer,
    totaldouble integer,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mld_fish_region (
    _expt_key integer NOT NULL,
    sequencenum integer NOT NULL,
    region text,
    totalsingle integer,
    totaldouble integer,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mld_hit (
    _expt_key integer NOT NULL,
    _probe_key integer NOT NULL,
    _target_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mld_hybrid (
    _expt_key integer NOT NULL,
    chrsorgenes smallint NOT NULL,
    band text,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mld_insitu (
    _expt_key integer NOT NULL,
    band text,
    _strain_key integer NOT NULL,
    cellorigin text,
    karyotype text,
    robertsonians text,
    nummetaphase integer,
    totalgrains integer,
    grainsonchrom integer,
    grainsotherchrom integer,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mld_isregion (
    _expt_key integer NOT NULL,
    sequencenum integer NOT NULL,
    region text,
    graincount integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mld_matrix (
    _expt_key integer NOT NULL,
    _cross_key integer NOT NULL,
    female text,
    female2 text,
    male text,
    male2 text,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mld_mc2point (
    _expt_key integer NOT NULL,
    _marker_key_1 integer NOT NULL,
    _marker_key_2 integer NOT NULL,
    sequencenum integer NOT NULL,
    numrecombinants integer NOT NULL,
    numparentals integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mld_mcdatalist (
    _expt_key integer NOT NULL,
    sequencenum integer NOT NULL,
    alleleline text NOT NULL,
    offspringnmbr integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mld_ri (
    _expt_key integer NOT NULL,
    ri_idlist text NOT NULL,
    _riset_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mld_ri2point (
    _expt_key integer NOT NULL,
    _marker_key_1 integer NOT NULL,
    _marker_key_2 integer NOT NULL,
    sequencenum integer NOT NULL,
    numrecombinants integer NOT NULL,
    numtotal integer NOT NULL,
    ri_lines text,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mld_ridata (
    _expt_key integer NOT NULL,
    _marker_key integer NOT NULL,
    sequencenum integer NOT NULL,
    alleleline text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mld_statistics (
    _expt_key integer NOT NULL,
    sequencenum integer NOT NULL,
    _marker_key_1 integer NOT NULL,
    _marker_key_2 integer NOT NULL,
    recomb integer NOT NULL,
    total integer NOT NULL,
    pcntrecomb numeric NOT NULL,
    stderr numeric NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mrk_types (
    _marker_type_key integer NOT NULL,
    name text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mrk_biotypemapping (
    _biotypemapping_key integer NOT NULL,
    _biotypevocab_key integer NOT NULL,
    _biotypeterm_key integer NOT NULL,
    _mcvterm_key integer NOT NULL,
    _primarymcvterm_key integer NOT NULL,
    _marker_type_key integer NOT NULL,
    usemcvchildren integer DEFAULT 0 NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mrk_cluster (
    _cluster_key integer DEFAULT nextval('mgd.mrk_cluster_seq'::regclass) NOT NULL,
    _clustertype_key integer NOT NULL,
    _clustersource_key integer NOT NULL,
    clusterid text,
    version text,
    cluster_date timestamp without time zone DEFAULT now(),
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mrk_clustermember (
    _clustermember_key integer DEFAULT nextval('mgd.mrk_clustermember_seq'::regclass) NOT NULL,
    _cluster_key integer NOT NULL,
    _marker_key integer NOT NULL,
    sequencenum integer NOT NULL
);


CREATE TABLE mgd.mrk_current (
    _current_key integer NOT NULL,
    _marker_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mrk_history (
    _assoc_key integer DEFAULT nextval('mgd.mrk_history_seq'::regclass) NOT NULL,
    _marker_key integer NOT NULL,
    _marker_event_key integer NOT NULL,
    _marker_eventreason_key integer NOT NULL,
    _history_key integer NOT NULL,
    _refs_key integer,
    sequencenum integer NOT NULL,
    name text,
    event_date timestamp without time zone DEFAULT now(),
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mrk_label (
    _label_key integer NOT NULL,
    _marker_key integer NOT NULL,
    _label_status_key integer NOT NULL,
    _organism_key integer NOT NULL,
    _orthologorganism_key integer,
    priority integer NOT NULL,
    label text NOT NULL,
    labeltype text NOT NULL,
    labeltypename text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mrk_location_cache (
    _marker_key integer NOT NULL,
    _marker_type_key integer NOT NULL,
    _organism_key integer NOT NULL,
    chromosome text NOT NULL,
    sequencenum integer NOT NULL,
    cytogeneticoffset text,
    cmoffset double precision,
    genomicchromosome text,
    startcoordinate double precision,
    endcoordinate double precision,
    strand character(1),
    mapunits text,
    provider text,
    version text,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mrk_status (
    _marker_status_key integer NOT NULL,
    status text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mrk_mcv_cache (
    _marker_key integer NOT NULL,
    _mcvterm_key integer NOT NULL,
    term text NOT NULL,
    qualifier character(1) NOT NULL,
    directterms text NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mrk_mcv_count_cache (
    _mcvterm_key integer NOT NULL,
    markercount integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.mrk_notes (
    _marker_key integer NOT NULL,
    note text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.prb_alias (
    _alias_key integer DEFAULT nextval('mgd.prb_alias_seq'::regclass) NOT NULL,
    _reference_key integer NOT NULL,
    alias text NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.prb_allele (
    _allele_key integer DEFAULT nextval('mgd.prb_allele_seq'::regclass) NOT NULL,
    _rflv_key integer NOT NULL,
    allele text NOT NULL,
    fragments text NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.prb_allele_strain (
    _allelestrain_key integer DEFAULT nextval('mgd.prb_allele_strain_seq'::regclass) NOT NULL,
    _allele_key integer NOT NULL,
    _strain_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.prb_marker (
    _assoc_key integer DEFAULT nextval('mgd.prb_marker_seq'::regclass) NOT NULL,
    _probe_key integer NOT NULL,
    _marker_key integer NOT NULL,
    _refs_key integer NOT NULL,
    relationship character(1),
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.prb_notes (
    _note_key integer DEFAULT nextval('mgd.prb_notes_seq'::regclass) NOT NULL,
    _probe_key integer NOT NULL,
    note text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.prb_ref_notes (
    _reference_key integer NOT NULL,
    note text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.prb_rflv (
    _rflv_key integer DEFAULT nextval('mgd.prb_rflv_seq'::regclass) NOT NULL,
    _reference_key integer NOT NULL,
    _marker_key integer NOT NULL,
    endonuclease text,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.prb_tissue (
    _tissue_key integer DEFAULT nextval('mgd.prb_tissue_seq'::regclass) NOT NULL,
    tissue text NOT NULL,
    standard smallint NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.prb_strain_genotype (
    _straingenotype_key integer DEFAULT nextval('mgd.prb_strain_genotype_seq'::regclass) NOT NULL,
    _strain_key integer NOT NULL,
    _genotype_key integer NOT NULL,
    _qualifier_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.prb_strain_marker (
    _strainmarker_key integer DEFAULT nextval('mgd.prb_strain_marker_seq'::regclass) NOT NULL,
    _strain_key integer NOT NULL,
    _marker_key integer,
    _allele_key integer,
    _qualifier_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.ri_riset (
    _riset_key integer NOT NULL,
    _strain_key_1 integer NOT NULL,
    _strain_key_2 integer NOT NULL,
    designation text NOT NULL,
    abbrev1 text NOT NULL,
    abbrev2 text NOT NULL,
    ri_idlist text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.ri_summary (
    _risummary_key integer NOT NULL,
    _riset_key integer NOT NULL,
    _marker_key integer NOT NULL,
    summary text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.ri_summary_expt_ref (
    _risummary_key integer NOT NULL,
    _expt_key integer NOT NULL,
    _refs_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.seq_allele_assoc (
    _assoc_key integer NOT NULL,
    _sequence_key integer NOT NULL,
    _allele_key integer NOT NULL,
    _qualifier_key integer NOT NULL,
    _refs_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.seq_coord_cache (
    _map_key integer NOT NULL,
    _sequence_key integer NOT NULL,
    chromosome text NOT NULL,
    startcoordinate numeric NOT NULL,
    endcoordinate numeric NOT NULL,
    strand character(1) NOT NULL,
    mapunits text NOT NULL,
    provider text NOT NULL,
    version text NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.seq_genemodel (
    _sequence_key integer NOT NULL,
    _gmmarker_type_key integer,
    rawbiotype text,
    exoncount integer,
    transcriptcount integer,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.seq_genetrap (
    _sequence_key integer NOT NULL,
    _tagmethod_key integer NOT NULL,
    _vectorend_key integer NOT NULL,
    _reversecomp_key integer NOT NULL,
    goodhitcount integer,
    pointcoordinate numeric,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.seq_marker_cache (
    _cache_key integer NOT NULL,
    _sequence_key integer NOT NULL,
    _marker_key integer NOT NULL,
    _organism_key integer NOT NULL,
    _refs_key integer NOT NULL,
    _qualifier_key integer NOT NULL,
    _sequenceprovider_key integer NOT NULL,
    _sequencetype_key integer NOT NULL,
    _logicaldb_key integer NOT NULL,
    _marker_type_key integer NOT NULL,
    _biotypeconflict_key integer NOT NULL,
    accid character varying(30) NOT NULL,
    rawbiotype text,
    annotation_date timestamp without time zone DEFAULT now() NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.seq_probe_cache (
    _sequence_key integer NOT NULL,
    _probe_key integer NOT NULL,
    _refs_key integer NOT NULL,
    annotation_date timestamp without time zone DEFAULT now() NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.seq_sequence (
    _sequence_key integer NOT NULL,
    _sequencetype_key integer NOT NULL,
    _sequencequality_key integer NOT NULL,
    _sequencestatus_key integer NOT NULL,
    _sequenceprovider_key integer NOT NULL,
    _organism_key integer NOT NULL,
    length integer,
    description text,
    version text,
    division character(3),
    virtual smallint NOT NULL,
    numberoforganisms integer,
    seqrecord_date timestamp without time zone DEFAULT now() NOT NULL,
    sequence_date timestamp without time zone DEFAULT now() NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.seq_sequence_assoc (
    _assoc_key integer NOT NULL,
    _sequence_key_1 integer NOT NULL,
    _qualifier_key integer NOT NULL,
    _sequence_key_2 integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.seq_sequence_raw (
    _sequence_key integer NOT NULL,
    rawtype text,
    rawlibrary text,
    raworganism text,
    rawstrain text,
    rawtissue text,
    rawage text,
    rawsex text,
    rawcellline text,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.seq_source_assoc (
    _assoc_key integer DEFAULT nextval('mgd.seq_source_assoc_seq'::regclass) NOT NULL,
    _sequence_key integer NOT NULL,
    _source_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.voc_allele_cache (
    _cache_key integer DEFAULT nextval('mgd.voc_allele_cache_seq'::regclass) NOT NULL,
    _term_key integer NOT NULL,
    _allele_key integer NOT NULL,
    annottype text NOT NULL
);


CREATE TABLE mgd.voc_annot_count_cache (
    _cache_key integer DEFAULT nextval('mgd.voc_annot_count_cache_seq'::regclass) NOT NULL,
    _term_key integer NOT NULL,
    _mgitype_key integer NOT NULL,
    objectcount integer NOT NULL,
    annotcount integer NOT NULL,
    annottype text NOT NULL
);


CREATE TABLE mgd.voc_evidence_property (
    _evidenceproperty_key integer DEFAULT nextval('mgd.voc_evidence_property_seq'::regclass) NOT NULL,
    _annotevidence_key integer NOT NULL,
    _propertyterm_key integer NOT NULL,
    stanza integer NOT NULL,
    sequencenum integer NOT NULL,
    value text NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.voc_marker_cache (
    _cache_key integer DEFAULT nextval('mgd.voc_marker_cache_seq'::regclass) NOT NULL,
    _term_key integer NOT NULL,
    _marker_key integer NOT NULL,
    annottype text NOT NULL
);


CREATE TABLE mgd.voc_term_emapa (
    _term_key integer NOT NULL,
    _defaultparent_key integer,
    startstage integer NOT NULL,
    endstage integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.voc_term_emaps (
    _term_key integer NOT NULL,
    _stage_key integer NOT NULL,
    _defaultparent_key integer,
    _emapa_term_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE mgd.wks_rosetta (
    _rosetta_key integer NOT NULL,
    _marker_key integer,
    wks_markersymbol text,
    wks_markerurl text,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


ALTER TABLE ONLY mgd.acc_accession
    ADD CONSTRAINT acc_accession_pkey PRIMARY KEY (_accession_key);


ALTER TABLE ONLY mgd.acc_accessionmax
    ADD CONSTRAINT acc_accessionmax_pkey PRIMARY KEY (prefixpart);


ALTER TABLE ONLY mgd.acc_accessionreference
    ADD CONSTRAINT acc_accessionreference_pkey PRIMARY KEY (_accession_key, _refs_key);


ALTER TABLE ONLY mgd.acc_actualdb
    ADD CONSTRAINT acc_actualdb_pkey PRIMARY KEY (_actualdb_key);


ALTER TABLE ONLY mgd.acc_logicaldb
    ADD CONSTRAINT acc_logicaldb_pkey PRIMARY KEY (_logicaldb_key);


ALTER TABLE ONLY mgd.acc_mgitype
    ADD CONSTRAINT acc_mgitype_pkey PRIMARY KEY (_mgitype_key);


ALTER TABLE ONLY mgd.all_allele_cellline
    ADD CONSTRAINT all_allele_cellline_pkey PRIMARY KEY (_assoc_key);


ALTER TABLE ONLY mgd.all_allele_mutation
    ADD CONSTRAINT all_allele_mutation_pkey PRIMARY KEY (_assoc_key);


ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele_pkey PRIMARY KEY (_allele_key);



ALTER TABLE ONLY mgd.all_cellline_derivation
    ADD CONSTRAINT all_cellline_derivation_pkey PRIMARY KEY (_derivation_key);



ALTER TABLE ONLY mgd.all_cellline
    ADD CONSTRAINT all_cellline_pkey PRIMARY KEY (_cellline_key);


ALTER TABLE ONLY mgd.all_cre_cache
    ADD CONSTRAINT all_cre_cache_pkey PRIMARY KEY (_cache_key);


ALTER TABLE ONLY mgd.all_knockout_cache
    ADD CONSTRAINT all_knockout_cache_pkey PRIMARY KEY (_knockout_key);


ALTER TABLE ONLY mgd.all_label
    ADD CONSTRAINT all_label_pkey PRIMARY KEY (_allele_key, priority, labeltype, label);



ALTER TABLE ONLY mgd.all_variant
    ADD CONSTRAINT all_variant_pkey PRIMARY KEY (_variant_key);


ALTER TABLE ONLY mgd.all_variant_sequence
    ADD CONSTRAINT all_variant_sequence_pkey PRIMARY KEY (_variantsequence_key);


ALTER TABLE ONLY mgd.bib_books
    ADD CONSTRAINT bib_books_pkey PRIMARY KEY (_refs_key);



ALTER TABLE ONLY mgd.bib_citation_cache
    ADD CONSTRAINT bib_citation_cache_pkey PRIMARY KEY (_refs_key);



ALTER TABLE ONLY mgd.bib_notes
    ADD CONSTRAINT bib_notes_pkey PRIMARY KEY (_refs_key);



ALTER TABLE ONLY mgd.bib_refs
    ADD CONSTRAINT bib_refs_pkey PRIMARY KEY (_refs_key);



ALTER TABLE ONLY mgd.bib_workflow_data
    ADD CONSTRAINT bib_workflow_data_pkey PRIMARY KEY (_assoc_key);



ALTER TABLE ONLY mgd.bib_workflow_relevance
    ADD CONSTRAINT bib_workflow_relevance_pkey PRIMARY KEY (_assoc_key);



ALTER TABLE ONLY mgd.bib_workflow_status
    ADD CONSTRAINT bib_workflow_status_pkey PRIMARY KEY (_assoc_key);



ALTER TABLE ONLY mgd.bib_workflow_tag
    ADD CONSTRAINT bib_workflow_tag_pkey PRIMARY KEY (_assoc_key);



ALTER TABLE ONLY mgd.crs_cross
    ADD CONSTRAINT crs_cross_pkey PRIMARY KEY (_cross_key);


ALTER TABLE ONLY mgd.crs_matrix
    ADD CONSTRAINT crs_matrix_pkey PRIMARY KEY (_cross_key, rownumber);


ALTER TABLE ONLY mgd.crs_progeny
    ADD CONSTRAINT crs_progeny_pkey PRIMARY KEY (_cross_key, sequencenum);


ALTER TABLE ONLY mgd.crs_references
    ADD CONSTRAINT crs_references_pkey PRIMARY KEY (_cross_key, _marker_key, _refs_key);


ALTER TABLE ONLY mgd.crs_typings
    ADD CONSTRAINT crs_typings_pkey PRIMARY KEY (_cross_key, rownumber, colnumber);


ALTER TABLE ONLY mgd.dag_closure
    ADD CONSTRAINT dag_closure_pkey PRIMARY KEY (_dag_key, _ancestor_key, _descendent_key);


ALTER TABLE ONLY mgd.dag_dag
    ADD CONSTRAINT dag_dag_pkey PRIMARY KEY (_dag_key);


ALTER TABLE ONLY mgd.dag_edge
    ADD CONSTRAINT dag_edge_pkey PRIMARY KEY (_edge_key);


ALTER TABLE ONLY mgd.dag_label
    ADD CONSTRAINT dag_label_pkey PRIMARY KEY (_label_key);


ALTER TABLE ONLY mgd.dag_node
    ADD CONSTRAINT dag_node_pkey PRIMARY KEY (_node_key);


ALTER TABLE ONLY mgd.go_tracking
    ADD CONSTRAINT go_tracking_pkey PRIMARY KEY (_marker_key);


ALTER TABLE ONLY mgd.gxd_allelegenotype
    ADD CONSTRAINT gxd_allelegenotype_pkey PRIMARY KEY (_genotype_key, _allele_key);


ALTER TABLE ONLY mgd.gxd_allelepair
    ADD CONSTRAINT gxd_allelepair_pkey PRIMARY KEY (_allelepair_key);


ALTER TABLE ONLY mgd.gxd_antibody
    ADD CONSTRAINT gxd_antibody_pkey PRIMARY KEY (_antibody_key);


ALTER TABLE ONLY mgd.gxd_antibodyalias
    ADD CONSTRAINT gxd_antibodyalias_pkey PRIMARY KEY (_antibodyalias_key);


ALTER TABLE ONLY mgd.gxd_antibodymarker
    ADD CONSTRAINT gxd_antibodymarker_pkey PRIMARY KEY (_antibodymarker_key);


ALTER TABLE ONLY mgd.gxd_antibodyprep
    ADD CONSTRAINT gxd_antibodyprep_pkey PRIMARY KEY (_antibodyprep_key);


ALTER TABLE ONLY mgd.gxd_antigen
    ADD CONSTRAINT gxd_antigen_pkey PRIMARY KEY (_antigen_key);


ALTER TABLE ONLY mgd.gxd_assay
    ADD CONSTRAINT gxd_assay_pkey PRIMARY KEY (_assay_key);


ALTER TABLE ONLY mgd.gxd_assaynote
    ADD CONSTRAINT gxd_assaynote_pkey PRIMARY KEY (_assaynote_key);


ALTER TABLE ONLY mgd.gxd_assaytype
    ADD CONSTRAINT gxd_assaytype_pkey PRIMARY KEY (_assaytype_key);


ALTER TABLE ONLY mgd.gxd_expression
    ADD CONSTRAINT gxd_expression_pkey PRIMARY KEY (_expression_key);



ALTER TABLE ONLY mgd.gxd_gelband
    ADD CONSTRAINT gxd_gelband_pkey PRIMARY KEY (_gelband_key);


ALTER TABLE ONLY mgd.gxd_gellane
    ADD CONSTRAINT gxd_gellane_pkey PRIMARY KEY (_gellane_key);


ALTER TABLE ONLY mgd.gxd_gellanestructure
    ADD CONSTRAINT gxd_gellanestructure_pkey PRIMARY KEY (_gellanestructure_key);


ALTER TABLE ONLY mgd.gxd_gelrow
    ADD CONSTRAINT gxd_gelrow_pkey PRIMARY KEY (_gelrow_key);


ALTER TABLE ONLY mgd.gxd_genotype
    ADD CONSTRAINT gxd_genotype_pkey PRIMARY KEY (_genotype_key);


ALTER TABLE ONLY mgd.gxd_htexperiment
    ADD CONSTRAINT gxd_htexperiment_pkey PRIMARY KEY (_experiment_key);


ALTER TABLE ONLY mgd.gxd_htexperimentvariable
    ADD CONSTRAINT gxd_htexperimentvariable_pkey PRIMARY KEY (_experimentvariable_key);


ALTER TABLE ONLY mgd.gxd_htrawsample
    ADD CONSTRAINT gxd_htrawsample_pkey PRIMARY KEY (_rawsample_key);


ALTER TABLE ONLY mgd.gxd_htsample
    ADD CONSTRAINT gxd_htsample_pkey PRIMARY KEY (_sample_key);


ALTER TABLE ONLY mgd.gxd_htsample_rnaseq
    ADD CONSTRAINT gxd_htsample_rnaseq_pkey PRIMARY KEY (_rnaseq_key);


ALTER TABLE ONLY mgd.gxd_htsample_rnaseqcombined
    ADD CONSTRAINT gxd_htsample_rnaseqcombined_pkey PRIMARY KEY (_rnaseqcombined_key);


ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset_cache
    ADD CONSTRAINT gxd_htsample_rnaseqset_cache_pkey PRIMARY KEY (_assoc_key);


ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset
    ADD CONSTRAINT gxd_htsample_rnaseqset_pkey PRIMARY KEY (_rnaseqset_key);


ALTER TABLE ONLY mgd.gxd_htsample_rnaseqsetmember
    ADD CONSTRAINT gxd_htsample_rnaseqsetmember_pkey PRIMARY KEY (_rnaseqsetmember_key);


ALTER TABLE ONLY mgd.gxd_index
    ADD CONSTRAINT gxd_index_pkey PRIMARY KEY (_index_key);


ALTER TABLE ONLY mgd.gxd_index_stages
    ADD CONSTRAINT gxd_index_stages_pkey PRIMARY KEY (_indexstage_key);


ALTER TABLE ONLY mgd.gxd_insituresult
    ADD CONSTRAINT gxd_insituresult_pkey PRIMARY KEY (_result_key);


ALTER TABLE ONLY mgd.gxd_insituresultimage
    ADD CONSTRAINT gxd_insituresultimage_pkey PRIMARY KEY (_resultimage_key);


ALTER TABLE ONLY mgd.gxd_isresultcelltype
    ADD CONSTRAINT gxd_isresultcelltype_pkey PRIMARY KEY (_resultcelltype_key);


ALTER TABLE ONLY mgd.gxd_isresultstructure
    ADD CONSTRAINT gxd_isresultstructure_pkey PRIMARY KEY (_resultstructure_key);


ALTER TABLE ONLY mgd.gxd_probeprep
    ADD CONSTRAINT gxd_probeprep_pkey PRIMARY KEY (_probeprep_key);


ALTER TABLE ONLY mgd.gxd_specimen
    ADD CONSTRAINT gxd_specimen_pkey PRIMARY KEY (_specimen_key);


ALTER TABLE ONLY mgd.gxd_theilerstage
    ADD CONSTRAINT gxd_theilerstage_pkey PRIMARY KEY (_stage_key);


ALTER TABLE ONLY mgd.img_image
    ADD CONSTRAINT img_image_pkey PRIMARY KEY (_image_key);


ALTER TABLE ONLY mgd.img_imagepane_assoc
    ADD CONSTRAINT img_imagepane_assoc_pkey PRIMARY KEY (_assoc_key);


ALTER TABLE ONLY mgd.img_imagepane
    ADD CONSTRAINT img_imagepane_pkey PRIMARY KEY (_imagepane_key);


ALTER TABLE ONLY mgd.map_coord_collection
    ADD CONSTRAINT map_coord_collection_pkey PRIMARY KEY (_collection_key);


ALTER TABLE ONLY mgd.map_coord_feature
    ADD CONSTRAINT map_coord_feature_pkey PRIMARY KEY (_feature_key);


ALTER TABLE ONLY mgd.map_coordinate
    ADD CONSTRAINT map_coordinate_pkey PRIMARY KEY (_map_key);


ALTER TABLE ONLY mgd.mgi_dbinfo
    ADD CONSTRAINT mgi_dbinfo_pkey PRIMARY KEY (public_version);


ALTER TABLE ONLY mgd.mgi_keyvalue
    ADD CONSTRAINT mgi_keyvalue_pkey PRIMARY KEY (_keyvalue_key);



ALTER TABLE ONLY mgd.mgi_note
    ADD CONSTRAINT mgi_note_pkey PRIMARY KEY (_note_key);



ALTER TABLE ONLY mgd.mgi_notetype
    ADD CONSTRAINT mgi_notetype_pkey PRIMARY KEY (_notetype_key);


ALTER TABLE ONLY mgd.mgi_organism_mgitype
    ADD CONSTRAINT mgi_organism_mgitype_pkey PRIMARY KEY (_assoc_key);


ALTER TABLE ONLY mgd.mgi_organism
    ADD CONSTRAINT mgi_organism_pkey PRIMARY KEY (_organism_key);


ALTER TABLE ONLY mgd.mgi_property
    ADD CONSTRAINT mgi_property_pkey PRIMARY KEY (_property_key);


ALTER TABLE ONLY mgd.mgi_propertytype
    ADD CONSTRAINT mgi_propertytype_pkey PRIMARY KEY (_propertytype_key);


ALTER TABLE ONLY mgd.mgi_refassoctype
    ADD CONSTRAINT mgi_refassoctype_pkey PRIMARY KEY (_refassoctype_key);


ALTER TABLE ONLY mgd.mgi_reference_assoc
    ADD CONSTRAINT mgi_reference_assoc_pkey PRIMARY KEY (_assoc_key);


ALTER TABLE ONLY mgd.mgi_relationship_category
    ADD CONSTRAINT mgi_relationship_category_pkey PRIMARY KEY (_category_key);



ALTER TABLE ONLY mgd.mgi_relationship
    ADD CONSTRAINT mgi_relationship_pkey PRIMARY KEY (_relationship_key);



ALTER TABLE ONLY mgd.mgi_relationship_property
    ADD CONSTRAINT mgi_relationship_property_pkey PRIMARY KEY (_relationshipproperty_key);



ALTER TABLE ONLY mgd.mgi_set
    ADD CONSTRAINT mgi_set_pkey PRIMARY KEY (_set_key);


ALTER TABLE ONLY mgd.mgi_setmember_emapa
    ADD CONSTRAINT mgi_setmember_emapa_pkey PRIMARY KEY (_setmember_emapa_key);


ALTER TABLE ONLY mgd.mgi_setmember
    ADD CONSTRAINT mgi_setmember_pkey PRIMARY KEY (_setmember_key);


ALTER TABLE ONLY mgd.mgi_synonym
    ADD CONSTRAINT mgi_synonym_pkey PRIMARY KEY (_synonym_key);


ALTER TABLE ONLY mgd.mgi_synonymtype
    ADD CONSTRAINT mgi_synonymtype_pkey PRIMARY KEY (_synonymtype_key);


ALTER TABLE ONLY mgd.mgi_translation
    ADD CONSTRAINT mgi_translation_pkey PRIMARY KEY (_translation_key);


ALTER TABLE ONLY mgd.mgi_translationtype
    ADD CONSTRAINT mgi_translationtype_pkey PRIMARY KEY (_translationtype_key);


ALTER TABLE ONLY mgd.mgi_user
    ADD CONSTRAINT mgi_user_pkey PRIMARY KEY (_user_key);


ALTER TABLE ONLY mgd.mld_assay_types
    ADD CONSTRAINT mld_assay_types_pkey PRIMARY KEY (_assay_type_key);



ALTER TABLE ONLY mgd.mld_concordance
    ADD CONSTRAINT mld_concordance_pkey PRIMARY KEY (_expt_key, sequencenum);



ALTER TABLE ONLY mgd.mld_contig
    ADD CONSTRAINT mld_contig_pkey PRIMARY KEY (_contig_key);



ALTER TABLE ONLY mgd.mld_contigprobe
    ADD CONSTRAINT mld_contigprobe_pkey PRIMARY KEY (_contig_key, sequencenum);



ALTER TABLE ONLY mgd.mld_expt_marker
    ADD CONSTRAINT mld_expt_marker_pkey PRIMARY KEY (_assoc_key);



ALTER TABLE ONLY mgd.mld_expt_notes
    ADD CONSTRAINT mld_expt_notes_pkey PRIMARY KEY (_expt_key);



ALTER TABLE ONLY mgd.mld_expts
    ADD CONSTRAINT mld_expts_pkey PRIMARY KEY (_expt_key);



ALTER TABLE ONLY mgd.mld_fish
    ADD CONSTRAINT mld_fish_pkey PRIMARY KEY (_expt_key);



ALTER TABLE ONLY mgd.mld_fish_region
    ADD CONSTRAINT mld_fish_region_pkey PRIMARY KEY (_expt_key, sequencenum);



ALTER TABLE ONLY mgd.mld_hit
    ADD CONSTRAINT mld_hit_pkey PRIMARY KEY (_expt_key, _probe_key, _target_key);



ALTER TABLE ONLY mgd.mld_hybrid
    ADD CONSTRAINT mld_hybrid_pkey PRIMARY KEY (_expt_key);



ALTER TABLE ONLY mgd.mld_insitu
    ADD CONSTRAINT mld_insitu_pkey PRIMARY KEY (_expt_key);



ALTER TABLE ONLY mgd.mld_isregion
    ADD CONSTRAINT mld_isregion_pkey PRIMARY KEY (_expt_key, sequencenum);



ALTER TABLE ONLY mgd.mld_matrix
    ADD CONSTRAINT mld_matrix_pkey PRIMARY KEY (_expt_key);



ALTER TABLE ONLY mgd.mld_mc2point
    ADD CONSTRAINT mld_mc2point_pkey PRIMARY KEY (_expt_key, sequencenum);



ALTER TABLE ONLY mgd.mld_mcdatalist
    ADD CONSTRAINT mld_mcdatalist_pkey PRIMARY KEY (_expt_key, sequencenum);



ALTER TABLE ONLY mgd.mld_notes
    ADD CONSTRAINT mld_notes_pkey PRIMARY KEY (_refs_key);



ALTER TABLE ONLY mgd.mld_ri2point
    ADD CONSTRAINT mld_ri2point_pkey PRIMARY KEY (_expt_key, sequencenum);



ALTER TABLE ONLY mgd.mld_ri
    ADD CONSTRAINT mld_ri_pkey PRIMARY KEY (_expt_key);



ALTER TABLE ONLY mgd.mld_ridata
    ADD CONSTRAINT mld_ridata_pkey PRIMARY KEY (_expt_key, sequencenum);



ALTER TABLE ONLY mgd.mld_statistics
    ADD CONSTRAINT mld_statistics_pkey PRIMARY KEY (_expt_key, sequencenum);



ALTER TABLE ONLY mgd.mrk_biotypemapping
    ADD CONSTRAINT mrk_biotypemapping_pkey PRIMARY KEY (_biotypemapping_key);


ALTER TABLE ONLY mgd.mrk_chromosome
    ADD CONSTRAINT mrk_chromosome_pkey PRIMARY KEY (_chromosome_key);


ALTER TABLE ONLY mgd.mrk_cluster
    ADD CONSTRAINT mrk_cluster_pkey PRIMARY KEY (_cluster_key);


ALTER TABLE ONLY mgd.mrk_clustermember
    ADD CONSTRAINT mrk_clustermember_pkey PRIMARY KEY (_clustermember_key);


ALTER TABLE ONLY mgd.mrk_current
    ADD CONSTRAINT mrk_current_pkey PRIMARY KEY (_current_key, _marker_key);


ALTER TABLE ONLY mgd.mrk_do_cache
    ADD CONSTRAINT mrk_do_cache_pkey PRIMARY KEY (_cache_key);



ALTER TABLE ONLY mgd.mrk_history
    ADD CONSTRAINT mrk_history_pkey PRIMARY KEY (_assoc_key);



ALTER TABLE ONLY mgd.mrk_label
    ADD CONSTRAINT mrk_label_pkey PRIMARY KEY (_label_key);



ALTER TABLE ONLY mgd.mrk_location_cache
    ADD CONSTRAINT mrk_location_cache_pkey PRIMARY KEY (_marker_key);



ALTER TABLE ONLY mgd.mrk_marker
    ADD CONSTRAINT mrk_marker_pkey PRIMARY KEY (_marker_key);


ALTER TABLE ONLY mgd.mrk_mcv_cache
    ADD CONSTRAINT mrk_mcv_cache_pkey PRIMARY KEY (_marker_key, _mcvterm_key);



ALTER TABLE ONLY mgd.mrk_mcv_count_cache
    ADD CONSTRAINT mrk_mcv_count_cache_pkey PRIMARY KEY (_mcvterm_key);



ALTER TABLE ONLY mgd.mrk_notes
    ADD CONSTRAINT mrk_notes_pkey PRIMARY KEY (_marker_key);


ALTER TABLE ONLY mgd.mrk_reference
    ADD CONSTRAINT mrk_reference_pkey PRIMARY KEY (_marker_key, _refs_key);



ALTER TABLE ONLY mgd.mrk_status
    ADD CONSTRAINT mrk_status_pkey PRIMARY KEY (_marker_status_key);


ALTER TABLE ONLY mgd.mrk_strainmarker
    ADD CONSTRAINT mrk_strainmarker_pkey PRIMARY KEY (_strainmarker_key);


ALTER TABLE ONLY mgd.mrk_types
    ADD CONSTRAINT mrk_types_pkey PRIMARY KEY (_marker_type_key);


ALTER TABLE ONLY mgd.prb_alias
    ADD CONSTRAINT prb_alias_pkey PRIMARY KEY (_alias_key);



ALTER TABLE ONLY mgd.prb_allele
    ADD CONSTRAINT prb_allele_pkey PRIMARY KEY (_allele_key);


ALTER TABLE ONLY mgd.prb_allele_strain
    ADD CONSTRAINT prb_allele_strain_pkey PRIMARY KEY (_allelestrain_key);


ALTER TABLE ONLY mgd.prb_marker
    ADD CONSTRAINT prb_marker_pkey PRIMARY KEY (_assoc_key);



ALTER TABLE ONLY mgd.prb_notes
    ADD CONSTRAINT prb_notes_pkey PRIMARY KEY (_note_key);



ALTER TABLE ONLY mgd.prb_probe
    ADD CONSTRAINT prb_probe_pkey PRIMARY KEY (_probe_key);



ALTER TABLE ONLY mgd.prb_ref_notes
    ADD CONSTRAINT prb_ref_notes_pkey PRIMARY KEY (_reference_key);



ALTER TABLE ONLY mgd.prb_reference
    ADD CONSTRAINT prb_reference_pkey PRIMARY KEY (_reference_key);



ALTER TABLE ONLY mgd.prb_rflv
    ADD CONSTRAINT prb_rflv_pkey PRIMARY KEY (_rflv_key);



ALTER TABLE ONLY mgd.prb_source
    ADD CONSTRAINT prb_source_pkey PRIMARY KEY (_source_key);


ALTER TABLE ONLY mgd.prb_strain_genotype
    ADD CONSTRAINT prb_strain_genotype_pkey PRIMARY KEY (_straingenotype_key);



ALTER TABLE ONLY mgd.prb_strain_marker
    ADD CONSTRAINT prb_strain_marker_pkey PRIMARY KEY (_strainmarker_key);



ALTER TABLE ONLY mgd.prb_strain
    ADD CONSTRAINT prb_strain_pkey PRIMARY KEY (_strain_key);


ALTER TABLE ONLY mgd.prb_tissue
    ADD CONSTRAINT prb_tissue_pkey PRIMARY KEY (_tissue_key);



ALTER TABLE ONLY mgd.ri_riset
    ADD CONSTRAINT ri_riset_pkey PRIMARY KEY (_riset_key);


ALTER TABLE ONLY mgd.ri_summary_expt_ref
    ADD CONSTRAINT ri_summary_expt_ref_pkey PRIMARY KEY (_risummary_key, _expt_key);


ALTER TABLE ONLY mgd.ri_summary
    ADD CONSTRAINT ri_summary_pkey PRIMARY KEY (_risummary_key);


ALTER TABLE ONLY mgd.seq_allele_assoc
    ADD CONSTRAINT seq_allele_assoc_pkey PRIMARY KEY (_assoc_key);


ALTER TABLE ONLY mgd.seq_coord_cache
    ADD CONSTRAINT seq_coord_cache_pkey PRIMARY KEY (_map_key, _sequence_key);



ALTER TABLE ONLY mgd.seq_genemodel
    ADD CONSTRAINT seq_genemodel_pkey PRIMARY KEY (_sequence_key);


ALTER TABLE ONLY mgd.seq_genetrap
    ADD CONSTRAINT seq_genetrap_pkey PRIMARY KEY (_sequence_key);


ALTER TABLE ONLY mgd.seq_marker_cache
    ADD CONSTRAINT seq_marker_cache_pkey PRIMARY KEY (_cache_key);



ALTER TABLE ONLY mgd.seq_probe_cache
    ADD CONSTRAINT seq_probe_cache_pkey PRIMARY KEY (_sequence_key, _probe_key, _refs_key);



ALTER TABLE ONLY mgd.seq_sequence_assoc
    ADD CONSTRAINT seq_sequence_assoc_pkey PRIMARY KEY (_assoc_key);


ALTER TABLE ONLY mgd.seq_sequence
    ADD CONSTRAINT seq_sequence_pkey PRIMARY KEY (_sequence_key);



ALTER TABLE ONLY mgd.seq_sequence_raw
    ADD CONSTRAINT seq_sequence_raw_pkey PRIMARY KEY (_sequence_key);


ALTER TABLE ONLY mgd.seq_source_assoc
    ADD CONSTRAINT seq_source_assoc_pkey PRIMARY KEY (_assoc_key);


ALTER TABLE ONLY mgd.voc_allele_cache
    ADD CONSTRAINT voc_allele_cache_pkey PRIMARY KEY (_cache_key);


ALTER TABLE ONLY mgd.voc_annot_count_cache
    ADD CONSTRAINT voc_annot_count_cache_pkey PRIMARY KEY (_cache_key);


ALTER TABLE ONLY mgd.voc_annot
    ADD CONSTRAINT voc_annot_pkey PRIMARY KEY (_annot_key);


ALTER TABLE ONLY mgd.voc_annotheader
    ADD CONSTRAINT voc_annotheader_pkey PRIMARY KEY (_annotheader_key);


ALTER TABLE ONLY mgd.voc_annottype
    ADD CONSTRAINT voc_annottype_pkey PRIMARY KEY (_annottype_key);


ALTER TABLE ONLY mgd.voc_evidence
    ADD CONSTRAINT voc_evidence_pkey PRIMARY KEY (_annotevidence_key);


ALTER TABLE ONLY mgd.voc_evidence_property
    ADD CONSTRAINT voc_evidence_property_pkey PRIMARY KEY (_evidenceproperty_key);



ALTER TABLE ONLY mgd.voc_marker_cache
    ADD CONSTRAINT voc_marker_cache_pkey PRIMARY KEY (_cache_key);


ALTER TABLE ONLY mgd.voc_term_emapa
    ADD CONSTRAINT voc_term_emapa_pkey PRIMARY KEY (_term_key);


ALTER TABLE ONLY mgd.voc_term_emaps
    ADD CONSTRAINT voc_term_emaps_pkey PRIMARY KEY (_term_key);


ALTER TABLE ONLY mgd.voc_term
    ADD CONSTRAINT voc_term_pkey PRIMARY KEY (_term_key);


ALTER TABLE ONLY mgd.voc_vocab
    ADD CONSTRAINT voc_vocab_pkey PRIMARY KEY (_vocab_key);


ALTER TABLE ONLY mgd.voc_vocabdag
    ADD CONSTRAINT voc_vocabdag_pkey PRIMARY KEY (_vocab_key, _dag_key);


ALTER TABLE ONLY mgd.wks_rosetta
    ADD CONSTRAINT wks_rosetta_pkey PRIMARY KEY (_rosetta_key);


ALTER TABLE ONLY mgd.acc_accession
    ADD CONSTRAINT acc_accession__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.acc_accession
    ADD CONSTRAINT acc_accession__logicaldb_key_fkey FOREIGN KEY (_logicaldb_key) REFERENCES mgd.acc_logicaldb(_logicaldb_key) DEFERRABLE;


ALTER TABLE ONLY mgd.acc_accession
    ADD CONSTRAINT acc_accession__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.acc_accession
    ADD CONSTRAINT acc_accession__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.acc_accessionreference
    ADD CONSTRAINT acc_accessionreference__accession_key_fkey FOREIGN KEY (_accession_key) REFERENCES mgd.acc_accession(_accession_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.acc_accessionreference
    ADD CONSTRAINT acc_accessionreference__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.acc_accessionreference
    ADD CONSTRAINT acc_accessionreference__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.acc_accessionreference
    ADD CONSTRAINT acc_accessionreference__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.acc_actualdb
    ADD CONSTRAINT acc_actualdb__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.acc_actualdb
    ADD CONSTRAINT acc_actualdb__logicaldb_key_fkey FOREIGN KEY (_logicaldb_key) REFERENCES mgd.acc_logicaldb(_logicaldb_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.acc_actualdb
    ADD CONSTRAINT acc_actualdb__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.acc_logicaldb
    ADD CONSTRAINT acc_logicaldb__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.acc_logicaldb
    ADD CONSTRAINT acc_logicaldb__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.acc_logicaldb
    ADD CONSTRAINT acc_logicaldb__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


ALTER TABLE ONLY mgd.acc_mgitype
    ADD CONSTRAINT acc_mgitype__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.acc_mgitype
    ADD CONSTRAINT acc_mgitype__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele__allele_status_key_fkey FOREIGN KEY (_allele_status_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele__allele_type_key_fkey FOREIGN KEY (_allele_type_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele__approvedby_key_fkey FOREIGN KEY (_approvedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele__collection_key_fkey FOREIGN KEY (_collection_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele__markerallele_status_key_fkey FOREIGN KEY (_markerallele_status_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele__mode_key_fkey FOREIGN KEY (_mode_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele__strain_key_fkey FOREIGN KEY (_strain_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele__transmission_key_fkey FOREIGN KEY (_transmission_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_allele_cellline
    ADD CONSTRAINT all_allele_cellline__allele_key_fkey FOREIGN KEY (_allele_key) REFERENCES mgd.all_allele(_allele_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.all_allele_cellline
    ADD CONSTRAINT all_allele_cellline__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_allele_cellline
    ADD CONSTRAINT all_allele_cellline__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_allele_cellline
    ADD CONSTRAINT all_allele_cellline__mutantcellline_key_fkey FOREIGN KEY (_mutantcellline_key) REFERENCES mgd.all_cellline(_cellline_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_allele_mutation
    ADD CONSTRAINT all_allele_mutation__allele_key_fkey FOREIGN KEY (_allele_key) REFERENCES mgd.all_allele(_allele_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.all_allele_mutation
    ADD CONSTRAINT all_allele_mutation__mutation_key_fkey FOREIGN KEY (_mutation_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_cellline
    ADD CONSTRAINT all_cellline__cellline_type_key_fkey FOREIGN KEY (_cellline_type_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_cellline
    ADD CONSTRAINT all_cellline__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_cellline
    ADD CONSTRAINT all_cellline__derivation_key_fkey FOREIGN KEY (_derivation_key) REFERENCES mgd.all_cellline_derivation(_derivation_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_cellline
    ADD CONSTRAINT all_cellline__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_cellline
    ADD CONSTRAINT all_cellline__strain_key_fkey FOREIGN KEY (_strain_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_cellline_derivation
    ADD CONSTRAINT all_cellline_derivation__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_cellline_derivation
    ADD CONSTRAINT all_cellline_derivation__creator_key_fkey FOREIGN KEY (_creator_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_cellline_derivation
    ADD CONSTRAINT all_cellline_derivation__derivationtype_key_fkey FOREIGN KEY (_derivationtype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_cellline_derivation
    ADD CONSTRAINT all_cellline_derivation__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_cellline_derivation
    ADD CONSTRAINT all_cellline_derivation__parentcellline_key_fkey FOREIGN KEY (_parentcellline_key) REFERENCES mgd.all_cellline(_cellline_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_cellline_derivation
    ADD CONSTRAINT all_cellline_derivation__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_cellline_derivation
    ADD CONSTRAINT all_cellline_derivation__vector_key_fkey FOREIGN KEY (_vector_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_cellline_derivation
    ADD CONSTRAINT all_cellline_derivation__vectortype_key_fkey FOREIGN KEY (_vectortype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_cre_cache
    ADD CONSTRAINT all_cre_cache__allele_key_fkey FOREIGN KEY (_allele_key) REFERENCES mgd.all_allele(_allele_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.all_cre_cache
    ADD CONSTRAINT all_cre_cache__allele_type_key_fkey FOREIGN KEY (_allele_type_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.all_cre_cache
    ADD CONSTRAINT all_cre_cache__assay_key_fkey FOREIGN KEY (_assay_key) REFERENCES mgd.gxd_assay(_assay_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.all_cre_cache
    ADD CONSTRAINT all_cre_cache__celltype_term_key_fkey FOREIGN KEY (_celltype_term_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.all_cre_cache
    ADD CONSTRAINT all_cre_cache__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_cre_cache
    ADD CONSTRAINT all_cre_cache__emapa_term_key_fkey FOREIGN KEY (_emapa_term_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.all_cre_cache
    ADD CONSTRAINT all_cre_cache__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_cre_cache
    ADD CONSTRAINT all_cre_cache__stage_key_fkey FOREIGN KEY (_stage_key) REFERENCES mgd.gxd_theilerstage(_stage_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.all_knockout_cache
    ADD CONSTRAINT all_knockout_cache__allele_key_fkey FOREIGN KEY (_allele_key) REFERENCES mgd.all_allele(_allele_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_knockout_cache
    ADD CONSTRAINT all_knockout_cache__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_knockout_cache
    ADD CONSTRAINT all_knockout_cache__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.all_knockout_cache
    ADD CONSTRAINT all_knockout_cache__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_label
    ADD CONSTRAINT all_label__allele_key_fkey FOREIGN KEY (_allele_key) REFERENCES mgd.all_allele(_allele_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.all_variant
    ADD CONSTRAINT all_variant__allele_key_fkey FOREIGN KEY (_allele_key) REFERENCES mgd.all_allele(_allele_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.all_variant
    ADD CONSTRAINT all_variant__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_variant
    ADD CONSTRAINT all_variant__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_variant
    ADD CONSTRAINT all_variant__sourcevariant_key_fkey FOREIGN KEY (_sourcevariant_key) REFERENCES mgd.all_variant(_variant_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_variant
    ADD CONSTRAINT all_variant__strain_key_fkey FOREIGN KEY (_strain_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_variant_sequence
    ADD CONSTRAINT all_variant_sequence__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_variant_sequence
    ADD CONSTRAINT all_variant_sequence__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_variant_sequence
    ADD CONSTRAINT all_variant_sequence__sequence_type_key_fkey FOREIGN KEY (_sequence_type_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.all_variant_sequence
    ADD CONSTRAINT all_variant_sequence__variant_key_fkey FOREIGN KEY (_variant_key) REFERENCES mgd.all_variant(_variant_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.bib_books
    ADD CONSTRAINT bib_books__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.bib_citation_cache
    ADD CONSTRAINT bib_citation_cache__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.bib_citation_cache
    ADD CONSTRAINT bib_citation_cache__relevance_key_fkey FOREIGN KEY (_relevance_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.bib_notes
    ADD CONSTRAINT bib_notes__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.bib_refs
    ADD CONSTRAINT bib_refs__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.bib_refs
    ADD CONSTRAINT bib_refs__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.bib_refs
    ADD CONSTRAINT bib_refs__referencetype_key_fkey FOREIGN KEY (_referencetype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.bib_workflow_data
    ADD CONSTRAINT bib_workflow_data__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.bib_workflow_data
    ADD CONSTRAINT bib_workflow_data__extractedtext_key_fkey FOREIGN KEY (_extractedtext_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.bib_workflow_data
    ADD CONSTRAINT bib_workflow_data__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.bib_workflow_data
    ADD CONSTRAINT bib_workflow_data__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.bib_workflow_data
    ADD CONSTRAINT bib_workflow_data__supplemental_key_fkey FOREIGN KEY (_supplemental_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.bib_workflow_relevance
    ADD CONSTRAINT bib_workflow_relevance__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.bib_workflow_relevance
    ADD CONSTRAINT bib_workflow_relevance__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.bib_workflow_relevance
    ADD CONSTRAINT bib_workflow_relevance__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.bib_workflow_relevance
    ADD CONSTRAINT bib_workflow_relevance__relevance_key_fkey FOREIGN KEY (_relevance_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.bib_workflow_status
    ADD CONSTRAINT bib_workflow_status__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.bib_workflow_status
    ADD CONSTRAINT bib_workflow_status__group_key_fkey FOREIGN KEY (_group_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.bib_workflow_status
    ADD CONSTRAINT bib_workflow_status__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.bib_workflow_status
    ADD CONSTRAINT bib_workflow_status__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.bib_workflow_status
    ADD CONSTRAINT bib_workflow_status__status_key_fkey FOREIGN KEY (_status_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.bib_workflow_tag
    ADD CONSTRAINT bib_workflow_tag__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.bib_workflow_tag
    ADD CONSTRAINT bib_workflow_tag__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.bib_workflow_tag
    ADD CONSTRAINT bib_workflow_tag__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.bib_workflow_tag
    ADD CONSTRAINT bib_workflow_tag__tag_key_fkey FOREIGN KEY (_tag_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.crs_cross
    ADD CONSTRAINT crs_cross__femalestrain_key_fkey FOREIGN KEY (_femalestrain_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


ALTER TABLE ONLY mgd.crs_cross
    ADD CONSTRAINT crs_cross__malestrain_key_fkey FOREIGN KEY (_malestrain_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


ALTER TABLE ONLY mgd.crs_cross
    ADD CONSTRAINT crs_cross__strainho_key_fkey FOREIGN KEY (_strainho_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


ALTER TABLE ONLY mgd.crs_cross
    ADD CONSTRAINT crs_cross__strainht_key_fkey FOREIGN KEY (_strainht_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


ALTER TABLE ONLY mgd.crs_matrix
    ADD CONSTRAINT crs_matrix__cross_key_fkey FOREIGN KEY (_cross_key) REFERENCES mgd.crs_cross(_cross_key) DEFERRABLE;


ALTER TABLE ONLY mgd.crs_matrix
    ADD CONSTRAINT crs_matrix__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


ALTER TABLE ONLY mgd.crs_progeny
    ADD CONSTRAINT crs_progeny__cross_key_fkey FOREIGN KEY (_cross_key) REFERENCES mgd.crs_cross(_cross_key) DEFERRABLE;


ALTER TABLE ONLY mgd.crs_references
    ADD CONSTRAINT crs_references__cross_key_fkey FOREIGN KEY (_cross_key) REFERENCES mgd.crs_cross(_cross_key) DEFERRABLE;


ALTER TABLE ONLY mgd.crs_references
    ADD CONSTRAINT crs_references__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


ALTER TABLE ONLY mgd.crs_references
    ADD CONSTRAINT crs_references__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


ALTER TABLE ONLY mgd.crs_typings
    ADD CONSTRAINT crs_typings__cross_key_fkey FOREIGN KEY (_cross_key, rownumber) REFERENCES mgd.crs_matrix(_cross_key, rownumber) DEFERRABLE;


ALTER TABLE ONLY mgd.dag_closure
    ADD CONSTRAINT dag_closure__ancestor_key_fkey FOREIGN KEY (_ancestor_key) REFERENCES mgd.dag_node(_node_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.dag_closure
    ADD CONSTRAINT dag_closure__ancestorlabel_key_fkey FOREIGN KEY (_ancestorlabel_key) REFERENCES mgd.dag_label(_label_key) DEFERRABLE;


ALTER TABLE ONLY mgd.dag_closure
    ADD CONSTRAINT dag_closure__dag_key_fkey FOREIGN KEY (_dag_key) REFERENCES mgd.dag_dag(_dag_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.dag_closure
    ADD CONSTRAINT dag_closure__descendent_key_fkey FOREIGN KEY (_descendent_key) REFERENCES mgd.dag_node(_node_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.dag_closure
    ADD CONSTRAINT dag_closure__descendentlabel_key_fkey FOREIGN KEY (_descendentlabel_key) REFERENCES mgd.dag_label(_label_key) DEFERRABLE;


ALTER TABLE ONLY mgd.dag_closure
    ADD CONSTRAINT dag_closure__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.dag_dag
    ADD CONSTRAINT dag_dag__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.dag_dag
    ADD CONSTRAINT dag_dag__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


ALTER TABLE ONLY mgd.dag_edge
    ADD CONSTRAINT dag_edge__child_key_fkey FOREIGN KEY (_child_key) REFERENCES mgd.dag_node(_node_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.dag_edge
    ADD CONSTRAINT dag_edge__dag_key_fkey FOREIGN KEY (_dag_key) REFERENCES mgd.dag_dag(_dag_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.dag_edge
    ADD CONSTRAINT dag_edge__label_key_fkey FOREIGN KEY (_label_key) REFERENCES mgd.dag_label(_label_key) DEFERRABLE;


ALTER TABLE ONLY mgd.dag_edge
    ADD CONSTRAINT dag_edge__parent_key_fkey FOREIGN KEY (_parent_key) REFERENCES mgd.dag_node(_node_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.dag_node
    ADD CONSTRAINT dag_node__dag_key_fkey FOREIGN KEY (_dag_key) REFERENCES mgd.dag_dag(_dag_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.dag_node
    ADD CONSTRAINT dag_node__label_key_fkey FOREIGN KEY (_label_key) REFERENCES mgd.dag_label(_label_key) DEFERRABLE;


ALTER TABLE ONLY mgd.go_tracking
    ADD CONSTRAINT go_tracking__completedby_key_fkey FOREIGN KEY (_completedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.go_tracking
    ADD CONSTRAINT go_tracking__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.go_tracking
    ADD CONSTRAINT go_tracking__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.go_tracking
    ADD CONSTRAINT go_tracking__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_allelegenotype
    ADD CONSTRAINT gxd_allelegenotype__allele_key_fkey FOREIGN KEY (_allele_key) REFERENCES mgd.all_allele(_allele_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_allelegenotype
    ADD CONSTRAINT gxd_allelegenotype__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_allelegenotype
    ADD CONSTRAINT gxd_allelegenotype__genotype_key_fkey FOREIGN KEY (_genotype_key) REFERENCES mgd.gxd_genotype(_genotype_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_allelegenotype
    ADD CONSTRAINT gxd_allelegenotype__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_allelegenotype
    ADD CONSTRAINT gxd_allelegenotype__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_allelepair
    ADD CONSTRAINT gxd_allelepair__allele_key_1_fkey FOREIGN KEY (_allele_key_1) REFERENCES mgd.all_allele(_allele_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_allelepair
    ADD CONSTRAINT gxd_allelepair__allele_key_2_fkey FOREIGN KEY (_allele_key_2) REFERENCES mgd.all_allele(_allele_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_allelepair
    ADD CONSTRAINT gxd_allelepair__compound_key_fkey FOREIGN KEY (_compound_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_allelepair
    ADD CONSTRAINT gxd_allelepair__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_allelepair
    ADD CONSTRAINT gxd_allelepair__genotype_key_fkey FOREIGN KEY (_genotype_key) REFERENCES mgd.gxd_genotype(_genotype_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_allelepair
    ADD CONSTRAINT gxd_allelepair__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_allelepair
    ADD CONSTRAINT gxd_allelepair__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_allelepair
    ADD CONSTRAINT gxd_allelepair__mutantcellline_key_1_fkey FOREIGN KEY (_mutantcellline_key_1) REFERENCES mgd.all_cellline(_cellline_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_allelepair
    ADD CONSTRAINT gxd_allelepair__mutantcellline_key_2_fkey FOREIGN KEY (_mutantcellline_key_2) REFERENCES mgd.all_cellline(_cellline_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_allelepair
    ADD CONSTRAINT gxd_allelepair__pairstate_key_fkey FOREIGN KEY (_pairstate_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_antibody
    ADD CONSTRAINT gxd_antibody__antibodyclass_key_fkey FOREIGN KEY (_antibodyclass_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_antibody
    ADD CONSTRAINT gxd_antibody__antibodytype_key_fkey FOREIGN KEY (_antibodytype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_antibody
    ADD CONSTRAINT gxd_antibody__antigen_key_fkey FOREIGN KEY (_antigen_key) REFERENCES mgd.gxd_antigen(_antigen_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_antibody
    ADD CONSTRAINT gxd_antibody__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_antibody
    ADD CONSTRAINT gxd_antibody__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_antibody
    ADD CONSTRAINT gxd_antibody__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_antibodyalias
    ADD CONSTRAINT gxd_antibodyalias__antibody_key_fkey FOREIGN KEY (_antibody_key) REFERENCES mgd.gxd_antibody(_antibody_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_antibodyalias
    ADD CONSTRAINT gxd_antibodyalias__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_antibodymarker
    ADD CONSTRAINT gxd_antibodymarker__antibody_key_fkey FOREIGN KEY (_antibody_key) REFERENCES mgd.gxd_antibody(_antibody_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_antibodymarker
    ADD CONSTRAINT gxd_antibodymarker__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_antibodyprep
    ADD CONSTRAINT gxd_antibodyprep__antibody_key_fkey FOREIGN KEY (_antibody_key) REFERENCES mgd.gxd_antibody(_antibody_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_antibodyprep
    ADD CONSTRAINT gxd_antibodyprep__label_key_fkey FOREIGN KEY (_label_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_antibodyprep
    ADD CONSTRAINT gxd_antibodyprep__secondary_key_fkey FOREIGN KEY (_secondary_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_antigen
    ADD CONSTRAINT gxd_antigen__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_antigen
    ADD CONSTRAINT gxd_antigen__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_antigen
    ADD CONSTRAINT gxd_antigen__source_key_fkey FOREIGN KEY (_source_key) REFERENCES mgd.prb_source(_source_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_assay
    ADD CONSTRAINT gxd_assay__antibodyprep_key_fkey FOREIGN KEY (_antibodyprep_key) REFERENCES mgd.gxd_antibodyprep(_antibodyprep_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_assay
    ADD CONSTRAINT gxd_assay__assaytype_key_fkey FOREIGN KEY (_assaytype_key) REFERENCES mgd.gxd_assaytype(_assaytype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_assay
    ADD CONSTRAINT gxd_assay__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_assay
    ADD CONSTRAINT gxd_assay__imagepane_key_fkey FOREIGN KEY (_imagepane_key) REFERENCES mgd.img_imagepane(_imagepane_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_assay
    ADD CONSTRAINT gxd_assay__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_assay
    ADD CONSTRAINT gxd_assay__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_assay
    ADD CONSTRAINT gxd_assay__probeprep_key_fkey FOREIGN KEY (_probeprep_key) REFERENCES mgd.gxd_probeprep(_probeprep_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_assay
    ADD CONSTRAINT gxd_assay__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_assay
    ADD CONSTRAINT gxd_assay__reportergene_key_fkey FOREIGN KEY (_reportergene_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_assaynote
    ADD CONSTRAINT gxd_assaynote__assay_key_fkey FOREIGN KEY (_assay_key) REFERENCES mgd.gxd_assay(_assay_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_expression
    ADD CONSTRAINT gxd_expression__assay_key_fkey FOREIGN KEY (_assay_key) REFERENCES mgd.gxd_assay(_assay_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_expression
    ADD CONSTRAINT gxd_expression__assaytype_key_fkey FOREIGN KEY (_assaytype_key) REFERENCES mgd.gxd_assaytype(_assaytype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_expression
    ADD CONSTRAINT gxd_expression__celltype_term_key_fkey FOREIGN KEY (_celltype_term_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_expression
    ADD CONSTRAINT gxd_expression__emapa_term_key_fkey FOREIGN KEY (_emapa_term_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_expression
    ADD CONSTRAINT gxd_expression__gellane_key_fkey FOREIGN KEY (_gellane_key) REFERENCES mgd.gxd_gellane(_gellane_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_expression
    ADD CONSTRAINT gxd_expression__genotype_key_fkey FOREIGN KEY (_genotype_key) REFERENCES mgd.gxd_genotype(_genotype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_expression
    ADD CONSTRAINT gxd_expression__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_expression
    ADD CONSTRAINT gxd_expression__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_expression
    ADD CONSTRAINT gxd_expression__specimen_key_fkey FOREIGN KEY (_specimen_key) REFERENCES mgd.gxd_specimen(_specimen_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_expression
    ADD CONSTRAINT gxd_expression__stage_key_fkey FOREIGN KEY (_stage_key) REFERENCES mgd.gxd_theilerstage(_stage_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_gelband
    ADD CONSTRAINT gxd_gelband__gellane_key_fkey FOREIGN KEY (_gellane_key) REFERENCES mgd.gxd_gellane(_gellane_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_gelband
    ADD CONSTRAINT gxd_gelband__gelrow_key_fkey FOREIGN KEY (_gelrow_key) REFERENCES mgd.gxd_gelrow(_gelrow_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_gelband
    ADD CONSTRAINT gxd_gelband__strength_key_fkey FOREIGN KEY (_strength_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_gellane
    ADD CONSTRAINT gxd_gellane__assay_key_fkey FOREIGN KEY (_assay_key) REFERENCES mgd.gxd_assay(_assay_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_gellane
    ADD CONSTRAINT gxd_gellane__gelcontrol_key_fkey FOREIGN KEY (_gelcontrol_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_gellane
    ADD CONSTRAINT gxd_gellane__gelrnatype_key_fkey FOREIGN KEY (_gelrnatype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_gellane
    ADD CONSTRAINT gxd_gellane__genotype_key_fkey FOREIGN KEY (_genotype_key) REFERENCES mgd.gxd_genotype(_genotype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_gellanestructure
    ADD CONSTRAINT gxd_gellanestructure__emapa_term_key_fkey FOREIGN KEY (_emapa_term_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_gellanestructure
    ADD CONSTRAINT gxd_gellanestructure__gellane_key_fkey FOREIGN KEY (_gellane_key) REFERENCES mgd.gxd_gellane(_gellane_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_gellanestructure
    ADD CONSTRAINT gxd_gellanestructure__stage_key_fkey FOREIGN KEY (_stage_key) REFERENCES mgd.gxd_theilerstage(_stage_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_gelrow
    ADD CONSTRAINT gxd_gelrow__assay_key_fkey FOREIGN KEY (_assay_key) REFERENCES mgd.gxd_assay(_assay_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_gelrow
    ADD CONSTRAINT gxd_gelrow__gelunits_key_fkey FOREIGN KEY (_gelunits_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_genotype
    ADD CONSTRAINT gxd_genotype__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_genotype
    ADD CONSTRAINT gxd_genotype__existsas_key_fkey FOREIGN KEY (_existsas_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_genotype
    ADD CONSTRAINT gxd_genotype__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_genotype
    ADD CONSTRAINT gxd_genotype__strain_key_fkey FOREIGN KEY (_strain_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htexperiment
    ADD CONSTRAINT gxd_htexperiment__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htexperiment
    ADD CONSTRAINT gxd_htexperiment__curationstate_key_fkey FOREIGN KEY (_curationstate_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htexperiment
    ADD CONSTRAINT gxd_htexperiment__evaluatedby_key_fkey FOREIGN KEY (_evaluatedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htexperiment
    ADD CONSTRAINT gxd_htexperiment__evaluationstate_key_fkey FOREIGN KEY (_evaluationstate_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htexperiment
    ADD CONSTRAINT gxd_htexperiment__experimenttype_key_fkey FOREIGN KEY (_experimenttype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htexperiment
    ADD CONSTRAINT gxd_htexperiment__initialcuratedby_key_fkey FOREIGN KEY (_initialcuratedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htexperiment
    ADD CONSTRAINT gxd_htexperiment__lastcuratedby_key_fkey FOREIGN KEY (_lastcuratedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htexperiment
    ADD CONSTRAINT gxd_htexperiment__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htexperiment
    ADD CONSTRAINT gxd_htexperiment__source_key_fkey FOREIGN KEY (_source_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htexperiment
    ADD CONSTRAINT gxd_htexperiment__studytype_key_fkey FOREIGN KEY (_studytype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htexperimentvariable
    ADD CONSTRAINT gxd_htexperimentvariable__experiment_key_fkey FOREIGN KEY (_experiment_key) REFERENCES mgd.gxd_htexperiment(_experiment_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htexperimentvariable
    ADD CONSTRAINT gxd_htexperimentvariable__term_key_fkey FOREIGN KEY (_term_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htrawsample
    ADD CONSTRAINT gxd_htrawsample__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htrawsample
    ADD CONSTRAINT gxd_htrawsample__experiment_key_fkey FOREIGN KEY (_experiment_key) REFERENCES mgd.gxd_htexperiment(_experiment_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htrawsample
    ADD CONSTRAINT gxd_htrawsample__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample
    ADD CONSTRAINT gxd_htsample__celltype_term_key_fkey FOREIGN KEY (_celltype_term_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample
    ADD CONSTRAINT gxd_htsample__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample
    ADD CONSTRAINT gxd_htsample__emapa_key_fkey FOREIGN KEY (_emapa_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample
    ADD CONSTRAINT gxd_htsample__experiment_key_fkey FOREIGN KEY (_experiment_key) REFERENCES mgd.gxd_htexperiment(_experiment_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample
    ADD CONSTRAINT gxd_htsample__genotype_key_fkey FOREIGN KEY (_genotype_key) REFERENCES mgd.gxd_genotype(_genotype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample
    ADD CONSTRAINT gxd_htsample__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample
    ADD CONSTRAINT gxd_htsample__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample
    ADD CONSTRAINT gxd_htsample__relevance_key_fkey FOREIGN KEY (_relevance_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample
    ADD CONSTRAINT gxd_htsample__sex_key_fkey FOREIGN KEY (_sex_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample
    ADD CONSTRAINT gxd_htsample__stage_key_fkey FOREIGN KEY (_stage_key) REFERENCES mgd.gxd_theilerstage(_stage_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample_rnaseq
    ADD CONSTRAINT gxd_htsample_rnaseq__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample_rnaseq
    ADD CONSTRAINT gxd_htsample_rnaseq__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample_rnaseq
    ADD CONSTRAINT gxd_htsample_rnaseq__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample_rnaseq
    ADD CONSTRAINT gxd_htsample_rnaseq__rnaseqcombined_key_fkey FOREIGN KEY (_rnaseqcombined_key) REFERENCES mgd.gxd_htsample_rnaseqcombined(_rnaseqcombined_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample_rnaseq
    ADD CONSTRAINT gxd_htsample_rnaseq__sample_key_fkey FOREIGN KEY (_sample_key) REFERENCES mgd.gxd_htsample(_sample_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample_rnaseqcombined
    ADD CONSTRAINT gxd_htsample_rnaseqcombined__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample_rnaseqcombined
    ADD CONSTRAINT gxd_htsample_rnaseqcombined__level_key_fkey FOREIGN KEY (_level_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample_rnaseqcombined
    ADD CONSTRAINT gxd_htsample_rnaseqcombined__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample_rnaseqcombined
    ADD CONSTRAINT gxd_htsample_rnaseqcombined__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset
    ADD CONSTRAINT gxd_htsample_rnaseqset__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset
    ADD CONSTRAINT gxd_htsample_rnaseqset__emapa_key_fkey FOREIGN KEY (_emapa_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset
    ADD CONSTRAINT gxd_htsample_rnaseqset__experiment_key_fkey FOREIGN KEY (_experiment_key) REFERENCES mgd.gxd_htexperiment(_experiment_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset
    ADD CONSTRAINT gxd_htsample_rnaseqset__genotype_key_fkey FOREIGN KEY (_genotype_key) REFERENCES mgd.gxd_genotype(_genotype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset
    ADD CONSTRAINT gxd_htsample_rnaseqset__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset
    ADD CONSTRAINT gxd_htsample_rnaseqset__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset
    ADD CONSTRAINT gxd_htsample_rnaseqset__sex_key_fkey FOREIGN KEY (_sex_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset
    ADD CONSTRAINT gxd_htsample_rnaseqset__stage_key_fkey FOREIGN KEY (_stage_key) REFERENCES mgd.gxd_theilerstage(_stage_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset_cache
    ADD CONSTRAINT gxd_htsample_rnaseqset_cache__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset_cache
    ADD CONSTRAINT gxd_htsample_rnaseqset_cache__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset_cache
    ADD CONSTRAINT gxd_htsample_rnaseqset_cache__rnaseqcombined_key_fkey FOREIGN KEY (_rnaseqcombined_key) REFERENCES mgd.gxd_htsample_rnaseqcombined(_rnaseqcombined_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset_cache
    ADD CONSTRAINT gxd_htsample_rnaseqset_cache__rnaseqset_key_fkey FOREIGN KEY (_rnaseqset_key) REFERENCES mgd.gxd_htsample_rnaseqset(_rnaseqset_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample_rnaseqsetmember
    ADD CONSTRAINT gxd_htsample_rnaseqsetmember__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample_rnaseqsetmember
    ADD CONSTRAINT gxd_htsample_rnaseqsetmember__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample_rnaseqsetmember
    ADD CONSTRAINT gxd_htsample_rnaseqsetmember__rnaseqset_key_fkey FOREIGN KEY (_rnaseqset_key) REFERENCES mgd.gxd_htsample_rnaseqset(_rnaseqset_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_htsample_rnaseqsetmember
    ADD CONSTRAINT gxd_htsample_rnaseqsetmember__sample_key_fkey FOREIGN KEY (_sample_key) REFERENCES mgd.gxd_htsample(_sample_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_index
    ADD CONSTRAINT gxd_index__conditionalmutants_key_fkey FOREIGN KEY (_conditionalmutants_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_index
    ADD CONSTRAINT gxd_index__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_index
    ADD CONSTRAINT gxd_index__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_index
    ADD CONSTRAINT gxd_index__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_index
    ADD CONSTRAINT gxd_index__priority_key_fkey FOREIGN KEY (_priority_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_index
    ADD CONSTRAINT gxd_index__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_index_stages
    ADD CONSTRAINT gxd_index_stages__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_index_stages
    ADD CONSTRAINT gxd_index_stages__index_key_fkey FOREIGN KEY (_index_key) REFERENCES mgd.gxd_index(_index_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_index_stages
    ADD CONSTRAINT gxd_index_stages__indexassay_key_fkey FOREIGN KEY (_indexassay_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_index_stages
    ADD CONSTRAINT gxd_index_stages__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_index_stages
    ADD CONSTRAINT gxd_index_stages__stageid_key_fkey FOREIGN KEY (_stageid_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_insituresult
    ADD CONSTRAINT gxd_insituresult__pattern_key_fkey FOREIGN KEY (_pattern_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_insituresult
    ADD CONSTRAINT gxd_insituresult__specimen_key_fkey FOREIGN KEY (_specimen_key) REFERENCES mgd.gxd_specimen(_specimen_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_insituresult
    ADD CONSTRAINT gxd_insituresult__strength_key_fkey FOREIGN KEY (_strength_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_insituresultimage
    ADD CONSTRAINT gxd_insituresultimage__imagepane_key_fkey FOREIGN KEY (_imagepane_key) REFERENCES mgd.img_imagepane(_imagepane_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_insituresultimage
    ADD CONSTRAINT gxd_insituresultimage__result_key_fkey FOREIGN KEY (_result_key) REFERENCES mgd.gxd_insituresult(_result_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_isresultcelltype
    ADD CONSTRAINT gxd_isresultcelltype__celltype_term_key_fkey FOREIGN KEY (_celltype_term_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_isresultcelltype
    ADD CONSTRAINT gxd_isresultcelltype__result_key_fkey FOREIGN KEY (_result_key) REFERENCES mgd.gxd_insituresult(_result_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_isresultstructure
    ADD CONSTRAINT gxd_isresultstructure__emapa_term_key_fkey FOREIGN KEY (_emapa_term_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_isresultstructure
    ADD CONSTRAINT gxd_isresultstructure__result_key_fkey FOREIGN KEY (_result_key) REFERENCES mgd.gxd_insituresult(_result_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_isresultstructure
    ADD CONSTRAINT gxd_isresultstructure__stage_key_fkey FOREIGN KEY (_stage_key) REFERENCES mgd.gxd_theilerstage(_stage_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_probeprep
    ADD CONSTRAINT gxd_probeprep__label_key_fkey FOREIGN KEY (_label_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_probeprep
    ADD CONSTRAINT gxd_probeprep__probe_key_fkey FOREIGN KEY (_probe_key) REFERENCES mgd.prb_probe(_probe_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_probeprep
    ADD CONSTRAINT gxd_probeprep__sense_key_fkey FOREIGN KEY (_sense_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_probeprep
    ADD CONSTRAINT gxd_probeprep__visualization_key_fkey FOREIGN KEY (_visualization_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_specimen
    ADD CONSTRAINT gxd_specimen__assay_key_fkey FOREIGN KEY (_assay_key) REFERENCES mgd.gxd_assay(_assay_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_specimen
    ADD CONSTRAINT gxd_specimen__embedding_key_fkey FOREIGN KEY (_embedding_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_specimen
    ADD CONSTRAINT gxd_specimen__fixation_key_fkey FOREIGN KEY (_fixation_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.gxd_specimen
    ADD CONSTRAINT gxd_specimen__genotype_key_fkey FOREIGN KEY (_genotype_key) REFERENCES mgd.gxd_genotype(_genotype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.img_image
    ADD CONSTRAINT img_image__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.img_image
    ADD CONSTRAINT img_image__imageclass_key_fkey FOREIGN KEY (_imageclass_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.img_image
    ADD CONSTRAINT img_image__imagetype_key_fkey FOREIGN KEY (_imagetype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.img_image
    ADD CONSTRAINT img_image__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.img_image
    ADD CONSTRAINT img_image__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


ALTER TABLE ONLY mgd.img_image
    ADD CONSTRAINT img_image__thumbnailimage_key_fkey FOREIGN KEY (_thumbnailimage_key) REFERENCES mgd.img_image(_image_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.img_imagepane
    ADD CONSTRAINT img_imagepane__image_key_fkey FOREIGN KEY (_image_key) REFERENCES mgd.img_image(_image_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.img_imagepane_assoc
    ADD CONSTRAINT img_imagepane_assoc__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.img_imagepane_assoc
    ADD CONSTRAINT img_imagepane_assoc__imagepane_key_fkey FOREIGN KEY (_imagepane_key) REFERENCES mgd.img_imagepane(_imagepane_key) DEFERRABLE;


ALTER TABLE ONLY mgd.img_imagepane_assoc
    ADD CONSTRAINT img_imagepane_assoc__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.img_imagepane_assoc
    ADD CONSTRAINT img_imagepane_assoc__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.map_coord_collection
    ADD CONSTRAINT map_coord_collection__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.map_coord_collection
    ADD CONSTRAINT map_coord_collection__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.map_coord_feature
    ADD CONSTRAINT map_coord_feature__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.map_coord_feature
    ADD CONSTRAINT map_coord_feature__map_key_fkey FOREIGN KEY (_map_key) REFERENCES mgd.map_coordinate(_map_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.map_coord_feature
    ADD CONSTRAINT map_coord_feature__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.map_coord_feature
    ADD CONSTRAINT map_coord_feature__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.map_coordinate
    ADD CONSTRAINT map_coordinate__collection_key_fkey FOREIGN KEY (_collection_key) REFERENCES mgd.map_coord_collection(_collection_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.map_coordinate
    ADD CONSTRAINT map_coordinate__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.map_coordinate
    ADD CONSTRAINT map_coordinate__maptype_key_fkey FOREIGN KEY (_maptype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.map_coordinate
    ADD CONSTRAINT map_coordinate__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.map_coordinate
    ADD CONSTRAINT map_coordinate__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.map_coordinate
    ADD CONSTRAINT map_coordinate__units_key_fkey FOREIGN KEY (_units_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_keyvalue
    ADD CONSTRAINT mgi_keyvalue__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_keyvalue
    ADD CONSTRAINT mgi_keyvalue__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_keyvalue
    ADD CONSTRAINT mgi_keyvalue__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_note
    ADD CONSTRAINT mgi_note__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_note
    ADD CONSTRAINT mgi_note__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_note
    ADD CONSTRAINT mgi_note__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_note
    ADD CONSTRAINT mgi_note__notetype_key_fkey FOREIGN KEY (_notetype_key) REFERENCES mgd.mgi_notetype(_notetype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_notetype
    ADD CONSTRAINT mgi_notetype__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_notetype
    ADD CONSTRAINT mgi_notetype__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_notetype
    ADD CONSTRAINT mgi_notetype__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_organism
    ADD CONSTRAINT mgi_organism__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_organism
    ADD CONSTRAINT mgi_organism__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_organism_mgitype
    ADD CONSTRAINT mgi_organism_mgitype__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_organism_mgitype
    ADD CONSTRAINT mgi_organism_mgitype__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_organism_mgitype
    ADD CONSTRAINT mgi_organism_mgitype__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_organism_mgitype
    ADD CONSTRAINT mgi_organism_mgitype__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_property
    ADD CONSTRAINT mgi_property__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_property
    ADD CONSTRAINT mgi_property__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_property
    ADD CONSTRAINT mgi_property__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_property
    ADD CONSTRAINT mgi_property__propertyterm_key_fkey FOREIGN KEY (_propertyterm_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_property
    ADD CONSTRAINT mgi_property__propertytype_key_fkey FOREIGN KEY (_propertytype_key) REFERENCES mgd.mgi_propertytype(_propertytype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_propertytype
    ADD CONSTRAINT mgi_propertytype__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_propertytype
    ADD CONSTRAINT mgi_propertytype__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_propertytype
    ADD CONSTRAINT mgi_propertytype__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_propertytype
    ADD CONSTRAINT mgi_propertytype__vocab_key_fkey FOREIGN KEY (_vocab_key) REFERENCES mgd.voc_vocab(_vocab_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_refassoctype
    ADD CONSTRAINT mgi_refassoctype__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_refassoctype
    ADD CONSTRAINT mgi_refassoctype__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_refassoctype
    ADD CONSTRAINT mgi_refassoctype__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_reference_assoc
    ADD CONSTRAINT mgi_reference_assoc__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_reference_assoc
    ADD CONSTRAINT mgi_reference_assoc__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_reference_assoc
    ADD CONSTRAINT mgi_reference_assoc__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_reference_assoc
    ADD CONSTRAINT mgi_reference_assoc__refassoctype_key_fkey FOREIGN KEY (_refassoctype_key) REFERENCES mgd.mgi_refassoctype(_refassoctype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_reference_assoc
    ADD CONSTRAINT mgi_reference_assoc__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_relationship
    ADD CONSTRAINT mgi_relationship__category_key_fkey FOREIGN KEY (_category_key) REFERENCES mgd.mgi_relationship_category(_category_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_relationship
    ADD CONSTRAINT mgi_relationship__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_relationship
    ADD CONSTRAINT mgi_relationship__evidence_key_fkey FOREIGN KEY (_evidence_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_relationship
    ADD CONSTRAINT mgi_relationship__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_relationship
    ADD CONSTRAINT mgi_relationship__qualifier_key_fkey FOREIGN KEY (_qualifier_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_relationship
    ADD CONSTRAINT mgi_relationship__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_relationship
    ADD CONSTRAINT mgi_relationship__relationshipterm_key_fkey FOREIGN KEY (_relationshipterm_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_relationship_category
    ADD CONSTRAINT mgi_relationship_category__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_relationship_category
    ADD CONSTRAINT mgi_relationship_category__evidencevocab_key_fkey FOREIGN KEY (_evidencevocab_key) REFERENCES mgd.voc_vocab(_vocab_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_relationship_category
    ADD CONSTRAINT mgi_relationship_category__mgitype_key_1_fkey FOREIGN KEY (_mgitype_key_1) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_relationship_category
    ADD CONSTRAINT mgi_relationship_category__mgitype_key_2_fkey FOREIGN KEY (_mgitype_key_2) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_relationship_category
    ADD CONSTRAINT mgi_relationship_category__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_relationship_category
    ADD CONSTRAINT mgi_relationship_category__qualifiervocab_key_fkey FOREIGN KEY (_qualifiervocab_key) REFERENCES mgd.voc_vocab(_vocab_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_relationship_category
    ADD CONSTRAINT mgi_relationship_category__relationshipdag_key_fkey FOREIGN KEY (_relationshipdag_key) REFERENCES mgd.dag_dag(_dag_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_relationship_category
    ADD CONSTRAINT mgi_relationship_category__relationshipvocab_key_fkey FOREIGN KEY (_relationshipvocab_key) REFERENCES mgd.voc_vocab(_vocab_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_relationship_property
    ADD CONSTRAINT mgi_relationship_property__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_relationship_property
    ADD CONSTRAINT mgi_relationship_property__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_relationship_property
    ADD CONSTRAINT mgi_relationship_property__propertyname_key_fkey FOREIGN KEY (_propertyname_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_relationship_property
    ADD CONSTRAINT mgi_relationship_property__relationship_key_fkey FOREIGN KEY (_relationship_key) REFERENCES mgd.mgi_relationship(_relationship_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_set
    ADD CONSTRAINT mgi_set__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_set
    ADD CONSTRAINT mgi_set__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_set
    ADD CONSTRAINT mgi_set__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_setmember
    ADD CONSTRAINT mgi_setmember__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_setmember
    ADD CONSTRAINT mgi_setmember__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_setmember
    ADD CONSTRAINT mgi_setmember__set_key_fkey FOREIGN KEY (_set_key) REFERENCES mgd.mgi_set(_set_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_setmember_emapa
    ADD CONSTRAINT mgi_setmember_emapa__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_setmember_emapa
    ADD CONSTRAINT mgi_setmember_emapa__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_setmember_emapa
    ADD CONSTRAINT mgi_setmember_emapa__setmember_key_fkey FOREIGN KEY (_setmember_key) REFERENCES mgd.mgi_setmember(_setmember_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_setmember_emapa
    ADD CONSTRAINT mgi_setmember_emapa__stage_key_fkey FOREIGN KEY (_stage_key) REFERENCES mgd.gxd_theilerstage(_stage_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_synonym
    ADD CONSTRAINT mgi_synonym__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_synonym
    ADD CONSTRAINT mgi_synonym__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_synonym
    ADD CONSTRAINT mgi_synonym__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_synonym
    ADD CONSTRAINT mgi_synonym__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_synonym
    ADD CONSTRAINT mgi_synonym__synonymtype_key_fkey FOREIGN KEY (_synonymtype_key) REFERENCES mgd.mgi_synonymtype(_synonymtype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_synonymtype
    ADD CONSTRAINT mgi_synonymtype__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_synonymtype
    ADD CONSTRAINT mgi_synonymtype__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_synonymtype
    ADD CONSTRAINT mgi_synonymtype__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_synonymtype
    ADD CONSTRAINT mgi_synonymtype__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_translation
    ADD CONSTRAINT mgi_translation__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_translation
    ADD CONSTRAINT mgi_translation__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_translation
    ADD CONSTRAINT mgi_translation__translationtype_key_fkey FOREIGN KEY (_translationtype_key) REFERENCES mgd.mgi_translationtype(_translationtype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_translationtype
    ADD CONSTRAINT mgi_translationtype__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_translationtype
    ADD CONSTRAINT mgi_translationtype__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_translationtype
    ADD CONSTRAINT mgi_translationtype__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_translationtype
    ADD CONSTRAINT mgi_translationtype__vocab_key_fkey FOREIGN KEY (_vocab_key) REFERENCES mgd.voc_vocab(_vocab_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_user
    ADD CONSTRAINT mgi_user__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_user
    ADD CONSTRAINT mgi_user__group_key_fkey FOREIGN KEY (_group_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_user
    ADD CONSTRAINT mgi_user__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_user
    ADD CONSTRAINT mgi_user__userstatus_key_fkey FOREIGN KEY (_userstatus_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mgi_user
    ADD CONSTRAINT mgi_user__usertype_key_fkey FOREIGN KEY (_usertype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mld_concordance
    ADD CONSTRAINT mld_concordance__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mld_concordance
    ADD CONSTRAINT mld_concordance__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mld_contig
    ADD CONSTRAINT mld_contig__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mld_contigprobe
    ADD CONSTRAINT mld_contigprobe__contig_key_fkey FOREIGN KEY (_contig_key) REFERENCES mgd.mld_contig(_contig_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mld_contigprobe
    ADD CONSTRAINT mld_contigprobe__probe_key_fkey FOREIGN KEY (_probe_key) REFERENCES mgd.prb_probe(_probe_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mld_expt_marker
    ADD CONSTRAINT mld_expt_marker__allele_key_fkey FOREIGN KEY (_allele_key) REFERENCES mgd.all_allele(_allele_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mld_expt_marker
    ADD CONSTRAINT mld_expt_marker__assay_type_key_fkey FOREIGN KEY (_assay_type_key) REFERENCES mgd.mld_assay_types(_assay_type_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mld_expt_marker
    ADD CONSTRAINT mld_expt_marker__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mld_expt_marker
    ADD CONSTRAINT mld_expt_marker__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mld_expt_notes
    ADD CONSTRAINT mld_expt_notes__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mld_expts
    ADD CONSTRAINT mld_expts__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mld_fish
    ADD CONSTRAINT mld_fish__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mld_fish
    ADD CONSTRAINT mld_fish__strain_key_fkey FOREIGN KEY (_strain_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mld_fish_region
    ADD CONSTRAINT mld_fish_region__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mld_hit
    ADD CONSTRAINT mld_hit__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mld_hit
    ADD CONSTRAINT mld_hit__probe_key_fkey FOREIGN KEY (_probe_key) REFERENCES mgd.prb_probe(_probe_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mld_hit
    ADD CONSTRAINT mld_hit__target_key_fkey FOREIGN KEY (_target_key) REFERENCES mgd.prb_probe(_probe_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mld_hybrid
    ADD CONSTRAINT mld_hybrid__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mld_insitu
    ADD CONSTRAINT mld_insitu__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mld_insitu
    ADD CONSTRAINT mld_insitu__strain_key_fkey FOREIGN KEY (_strain_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mld_isregion
    ADD CONSTRAINT mld_isregion__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mld_matrix
    ADD CONSTRAINT mld_matrix__cross_key_fkey FOREIGN KEY (_cross_key) REFERENCES mgd.crs_cross(_cross_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mld_matrix
    ADD CONSTRAINT mld_matrix__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mld_mc2point
    ADD CONSTRAINT mld_mc2point__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mld_mc2point
    ADD CONSTRAINT mld_mc2point__marker_key_1_fkey FOREIGN KEY (_marker_key_1) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mld_mc2point
    ADD CONSTRAINT mld_mc2point__marker_key_2_fkey FOREIGN KEY (_marker_key_2) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mld_mcdatalist
    ADD CONSTRAINT mld_mcdatalist__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mld_notes
    ADD CONSTRAINT mld_notes__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mld_ri2point
    ADD CONSTRAINT mld_ri2point__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mld_ri2point
    ADD CONSTRAINT mld_ri2point__marker_key_1_fkey FOREIGN KEY (_marker_key_1) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mld_ri2point
    ADD CONSTRAINT mld_ri2point__marker_key_2_fkey FOREIGN KEY (_marker_key_2) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mld_ri
    ADD CONSTRAINT mld_ri__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mld_ridata
    ADD CONSTRAINT mld_ridata__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mld_ridata
    ADD CONSTRAINT mld_ridata__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mld_statistics
    ADD CONSTRAINT mld_statistics__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mld_statistics
    ADD CONSTRAINT mld_statistics__marker_key_1_fkey FOREIGN KEY (_marker_key_1) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mld_statistics
    ADD CONSTRAINT mld_statistics__marker_key_2_fkey FOREIGN KEY (_marker_key_2) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_biotypemapping
    ADD CONSTRAINT mrk_biotypemapping__biotypeterm_key_fkey FOREIGN KEY (_biotypeterm_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_biotypemapping
    ADD CONSTRAINT mrk_biotypemapping__biotypevocab_key_fkey FOREIGN KEY (_biotypevocab_key) REFERENCES mgd.voc_vocab(_vocab_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_biotypemapping
    ADD CONSTRAINT mrk_biotypemapping__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_biotypemapping
    ADD CONSTRAINT mrk_biotypemapping__marker_type_key_fkey FOREIGN KEY (_marker_type_key) REFERENCES mgd.mrk_types(_marker_type_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_biotypemapping
    ADD CONSTRAINT mrk_biotypemapping__mcvterm_key_fkey FOREIGN KEY (_mcvterm_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_biotypemapping
    ADD CONSTRAINT mrk_biotypemapping__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_biotypemapping
    ADD CONSTRAINT mrk_biotypemapping__primarymcvterm_key_fkey FOREIGN KEY (_primarymcvterm_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_chromosome
    ADD CONSTRAINT mrk_chromosome__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_chromosome
    ADD CONSTRAINT mrk_chromosome__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_chromosome
    ADD CONSTRAINT mrk_chromosome__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_cluster
    ADD CONSTRAINT mrk_cluster__clustersource_key_fkey FOREIGN KEY (_clustersource_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_cluster
    ADD CONSTRAINT mrk_cluster__clustertype_key_fkey FOREIGN KEY (_clustertype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_cluster
    ADD CONSTRAINT mrk_cluster__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_cluster
    ADD CONSTRAINT mrk_cluster__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_clustermember
    ADD CONSTRAINT mrk_clustermember__cluster_key_fkey FOREIGN KEY (_cluster_key) REFERENCES mgd.mrk_cluster(_cluster_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_clustermember
    ADD CONSTRAINT mrk_clustermember__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_current
    ADD CONSTRAINT mrk_current__current_key_fkey FOREIGN KEY (_current_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_current
    ADD CONSTRAINT mrk_current__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_do_cache
    ADD CONSTRAINT mrk_do_cache__genotype_key_fkey FOREIGN KEY (_genotype_key) REFERENCES mgd.gxd_genotype(_genotype_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_do_cache
    ADD CONSTRAINT mrk_do_cache__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_do_cache
    ADD CONSTRAINT mrk_do_cache__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_do_cache
    ADD CONSTRAINT mrk_do_cache__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_do_cache
    ADD CONSTRAINT mrk_do_cache__term_key_fkey FOREIGN KEY (_term_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_history
    ADD CONSTRAINT mrk_history__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_history
    ADD CONSTRAINT mrk_history__history_key_fkey FOREIGN KEY (_history_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_history
    ADD CONSTRAINT mrk_history__marker_event_key_fkey FOREIGN KEY (_marker_event_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_history
    ADD CONSTRAINT mrk_history__marker_eventreason_key_fkey FOREIGN KEY (_marker_eventreason_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_history
    ADD CONSTRAINT mrk_history__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_history
    ADD CONSTRAINT mrk_history__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_history
    ADD CONSTRAINT mrk_history__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_label
    ADD CONSTRAINT mrk_label__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_label
    ADD CONSTRAINT mrk_label__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_label
    ADD CONSTRAINT mrk_label__orthologorganism_key_fkey FOREIGN KEY (_orthologorganism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_location_cache
    ADD CONSTRAINT mrk_location_cache__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_location_cache
    ADD CONSTRAINT mrk_location_cache__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_location_cache
    ADD CONSTRAINT mrk_location_cache__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_location_cache
    ADD CONSTRAINT mrk_location_cache__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_marker
    ADD CONSTRAINT mrk_marker__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_marker
    ADD CONSTRAINT mrk_marker__marker_status_key_fkey FOREIGN KEY (_marker_status_key) REFERENCES mgd.mrk_status(_marker_status_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_marker
    ADD CONSTRAINT mrk_marker__marker_type_key_fkey FOREIGN KEY (_marker_type_key) REFERENCES mgd.mrk_types(_marker_type_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_marker
    ADD CONSTRAINT mrk_marker__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_marker
    ADD CONSTRAINT mrk_marker__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_mcv_cache
    ADD CONSTRAINT mrk_mcv_cache__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_mcv_cache
    ADD CONSTRAINT mrk_mcv_cache__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_mcv_cache
    ADD CONSTRAINT mrk_mcv_cache__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_mcv_count_cache
    ADD CONSTRAINT mrk_mcv_count_cache__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_mcv_count_cache
    ADD CONSTRAINT mrk_mcv_count_cache__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_notes
    ADD CONSTRAINT mrk_notes__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_reference
    ADD CONSTRAINT mrk_reference__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_reference
    ADD CONSTRAINT mrk_reference__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_strainmarker
    ADD CONSTRAINT mrk_strainmarker__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_strainmarker
    ADD CONSTRAINT mrk_strainmarker__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_strainmarker
    ADD CONSTRAINT mrk_strainmarker__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_strainmarker
    ADD CONSTRAINT mrk_strainmarker__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


ALTER TABLE ONLY mgd.mrk_strainmarker
    ADD CONSTRAINT mrk_strainmarker__strain_key_fkey FOREIGN KEY (_strain_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_alias
    ADD CONSTRAINT prb_alias__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_alias
    ADD CONSTRAINT prb_alias__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_alias
    ADD CONSTRAINT prb_alias__reference_key_fkey FOREIGN KEY (_reference_key) REFERENCES mgd.prb_reference(_reference_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.prb_allele
    ADD CONSTRAINT prb_allele__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_allele
    ADD CONSTRAINT prb_allele__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_allele
    ADD CONSTRAINT prb_allele__rflv_key_fkey FOREIGN KEY (_rflv_key) REFERENCES mgd.prb_rflv(_rflv_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.prb_allele_strain
    ADD CONSTRAINT prb_allele_strain__allele_key_fkey FOREIGN KEY (_allele_key) REFERENCES mgd.prb_allele(_allele_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.prb_allele_strain
    ADD CONSTRAINT prb_allele_strain__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_allele_strain
    ADD CONSTRAINT prb_allele_strain__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_allele_strain
    ADD CONSTRAINT prb_allele_strain__strain_key_fkey FOREIGN KEY (_strain_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_marker
    ADD CONSTRAINT prb_marker__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_marker
    ADD CONSTRAINT prb_marker__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_marker
    ADD CONSTRAINT prb_marker__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_marker
    ADD CONSTRAINT prb_marker__probe_key_fkey FOREIGN KEY (_probe_key) REFERENCES mgd.prb_probe(_probe_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.prb_marker
    ADD CONSTRAINT prb_marker__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_notes
    ADD CONSTRAINT prb_notes__probe_key_fkey FOREIGN KEY (_probe_key) REFERENCES mgd.prb_probe(_probe_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.prb_probe
    ADD CONSTRAINT prb_probe__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_probe
    ADD CONSTRAINT prb_probe__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_probe
    ADD CONSTRAINT prb_probe__segmenttype_key_fkey FOREIGN KEY (_segmenttype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_probe
    ADD CONSTRAINT prb_probe__source_key_fkey FOREIGN KEY (_source_key) REFERENCES mgd.prb_source(_source_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_probe
    ADD CONSTRAINT prb_probe__vector_key_fkey FOREIGN KEY (_vector_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_probe
    ADD CONSTRAINT prb_probe_ampprimer_fkey FOREIGN KEY (ampprimer) REFERENCES mgd.prb_probe(_probe_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_probe
    ADD CONSTRAINT prb_probe_derivedfrom_fkey FOREIGN KEY (derivedfrom) REFERENCES mgd.prb_probe(_probe_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_ref_notes
    ADD CONSTRAINT prb_ref_notes__reference_key_fkey FOREIGN KEY (_reference_key) REFERENCES mgd.prb_reference(_reference_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.prb_reference
    ADD CONSTRAINT prb_reference__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_reference
    ADD CONSTRAINT prb_reference__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_reference
    ADD CONSTRAINT prb_reference__probe_key_fkey FOREIGN KEY (_probe_key) REFERENCES mgd.prb_probe(_probe_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.prb_reference
    ADD CONSTRAINT prb_reference__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_rflv
    ADD CONSTRAINT prb_rflv__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_rflv
    ADD CONSTRAINT prb_rflv__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_rflv
    ADD CONSTRAINT prb_rflv__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_rflv
    ADD CONSTRAINT prb_rflv__reference_key_fkey FOREIGN KEY (_reference_key) REFERENCES mgd.prb_reference(_reference_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.prb_source
    ADD CONSTRAINT prb_source__cellline_key_fkey FOREIGN KEY (_cellline_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_source
    ADD CONSTRAINT prb_source__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_source
    ADD CONSTRAINT prb_source__gender_key_fkey FOREIGN KEY (_gender_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_source
    ADD CONSTRAINT prb_source__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_source
    ADD CONSTRAINT prb_source__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_source
    ADD CONSTRAINT prb_source__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_source
    ADD CONSTRAINT prb_source__segmenttype_key_fkey FOREIGN KEY (_segmenttype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_source
    ADD CONSTRAINT prb_source__strain_key_fkey FOREIGN KEY (_strain_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_source
    ADD CONSTRAINT prb_source__tissue_key_fkey FOREIGN KEY (_tissue_key) REFERENCES mgd.prb_tissue(_tissue_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_source
    ADD CONSTRAINT prb_source__vector_key_fkey FOREIGN KEY (_vector_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_strain
    ADD CONSTRAINT prb_strain__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_strain
    ADD CONSTRAINT prb_strain__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_strain
    ADD CONSTRAINT prb_strain__species_key_fkey FOREIGN KEY (_species_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_strain
    ADD CONSTRAINT prb_strain__straintype_key_fkey FOREIGN KEY (_straintype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_strain_genotype
    ADD CONSTRAINT prb_strain_genotype__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_strain_genotype
    ADD CONSTRAINT prb_strain_genotype__genotype_key_fkey FOREIGN KEY (_genotype_key) REFERENCES mgd.gxd_genotype(_genotype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_strain_genotype
    ADD CONSTRAINT prb_strain_genotype__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_strain_genotype
    ADD CONSTRAINT prb_strain_genotype__qualifier_key_fkey FOREIGN KEY (_qualifier_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_strain_genotype
    ADD CONSTRAINT prb_strain_genotype__strain_key_fkey FOREIGN KEY (_strain_key) REFERENCES mgd.prb_strain(_strain_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.prb_strain_marker
    ADD CONSTRAINT prb_strain_marker__allele_key_fkey FOREIGN KEY (_allele_key) REFERENCES mgd.all_allele(_allele_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_strain_marker
    ADD CONSTRAINT prb_strain_marker__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_strain_marker
    ADD CONSTRAINT prb_strain_marker__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_strain_marker
    ADD CONSTRAINT prb_strain_marker__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_strain_marker
    ADD CONSTRAINT prb_strain_marker__qualifier_key_fkey FOREIGN KEY (_qualifier_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.prb_strain_marker
    ADD CONSTRAINT prb_strain_marker__strain_key_fkey FOREIGN KEY (_strain_key) REFERENCES mgd.prb_strain(_strain_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.ri_riset
    ADD CONSTRAINT ri_riset__strain_key_1_fkey FOREIGN KEY (_strain_key_1) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


ALTER TABLE ONLY mgd.ri_riset
    ADD CONSTRAINT ri_riset__strain_key_2_fkey FOREIGN KEY (_strain_key_2) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


ALTER TABLE ONLY mgd.ri_summary
    ADD CONSTRAINT ri_summary__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


ALTER TABLE ONLY mgd.ri_summary
    ADD CONSTRAINT ri_summary__riset_key_fkey FOREIGN KEY (_riset_key) REFERENCES mgd.ri_riset(_riset_key) DEFERRABLE;


ALTER TABLE ONLY mgd.ri_summary_expt_ref
    ADD CONSTRAINT ri_summary_expt_ref__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) DEFERRABLE;


ALTER TABLE ONLY mgd.ri_summary_expt_ref
    ADD CONSTRAINT ri_summary_expt_ref__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_allele_assoc
    ADD CONSTRAINT seq_allele_assoc__allele_key_fkey FOREIGN KEY (_allele_key) REFERENCES mgd.all_allele(_allele_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.seq_allele_assoc
    ADD CONSTRAINT seq_allele_assoc__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_allele_assoc
    ADD CONSTRAINT seq_allele_assoc__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_allele_assoc
    ADD CONSTRAINT seq_allele_assoc__qualifier_key_fkey FOREIGN KEY (_qualifier_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_allele_assoc
    ADD CONSTRAINT seq_allele_assoc__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_allele_assoc
    ADD CONSTRAINT seq_allele_assoc__sequence_key_fkey FOREIGN KEY (_sequence_key) REFERENCES mgd.seq_sequence(_sequence_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.seq_coord_cache
    ADD CONSTRAINT seq_coord_cache__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_coord_cache
    ADD CONSTRAINT seq_coord_cache__map_key_fkey FOREIGN KEY (_map_key) REFERENCES mgd.map_coordinate(_map_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.seq_coord_cache
    ADD CONSTRAINT seq_coord_cache__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_coord_cache
    ADD CONSTRAINT seq_coord_cache__sequence_key_fkey FOREIGN KEY (_sequence_key) REFERENCES mgd.seq_sequence(_sequence_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.seq_genemodel
    ADD CONSTRAINT seq_genemodel__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_genemodel
    ADD CONSTRAINT seq_genemodel__gmmarker_type_key_fkey FOREIGN KEY (_gmmarker_type_key) REFERENCES mgd.mrk_types(_marker_type_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_genemodel
    ADD CONSTRAINT seq_genemodel__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_genemodel
    ADD CONSTRAINT seq_genemodel__sequence_key_fkey FOREIGN KEY (_sequence_key) REFERENCES mgd.seq_sequence(_sequence_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.seq_genetrap
    ADD CONSTRAINT seq_genetrap__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_genetrap
    ADD CONSTRAINT seq_genetrap__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_genetrap
    ADD CONSTRAINT seq_genetrap__reversecomp_key_fkey FOREIGN KEY (_reversecomp_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_genetrap
    ADD CONSTRAINT seq_genetrap__sequence_key_fkey FOREIGN KEY (_sequence_key) REFERENCES mgd.seq_sequence(_sequence_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.seq_genetrap
    ADD CONSTRAINT seq_genetrap__tagmethod_key_fkey FOREIGN KEY (_tagmethod_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_genetrap
    ADD CONSTRAINT seq_genetrap__vectorend_key_fkey FOREIGN KEY (_vectorend_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_marker_cache
    ADD CONSTRAINT seq_marker_cache__biotypeconflict_key_fkey FOREIGN KEY (_biotypeconflict_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.seq_marker_cache
    ADD CONSTRAINT seq_marker_cache__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_marker_cache
    ADD CONSTRAINT seq_marker_cache__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.seq_marker_cache
    ADD CONSTRAINT seq_marker_cache__marker_type_key_fkey FOREIGN KEY (_marker_type_key) REFERENCES mgd.mrk_types(_marker_type_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_marker_cache
    ADD CONSTRAINT seq_marker_cache__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_marker_cache
    ADD CONSTRAINT seq_marker_cache__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_marker_cache
    ADD CONSTRAINT seq_marker_cache__qualifier_key_fkey FOREIGN KEY (_qualifier_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.seq_marker_cache
    ADD CONSTRAINT seq_marker_cache__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.seq_marker_cache
    ADD CONSTRAINT seq_marker_cache__sequence_key_fkey FOREIGN KEY (_sequence_key) REFERENCES mgd.seq_sequence(_sequence_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.seq_marker_cache
    ADD CONSTRAINT seq_marker_cache__sequenceprovider_key_fkey FOREIGN KEY (_sequenceprovider_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.seq_marker_cache
    ADD CONSTRAINT seq_marker_cache__sequencetype_key_fkey FOREIGN KEY (_sequencetype_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.seq_probe_cache
    ADD CONSTRAINT seq_probe_cache__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_probe_cache
    ADD CONSTRAINT seq_probe_cache__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_probe_cache
    ADD CONSTRAINT seq_probe_cache__probe_key_fkey FOREIGN KEY (_probe_key) REFERENCES mgd.prb_probe(_probe_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_probe_cache
    ADD CONSTRAINT seq_probe_cache__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.seq_probe_cache
    ADD CONSTRAINT seq_probe_cache__sequence_key_fkey FOREIGN KEY (_sequence_key) REFERENCES mgd.seq_sequence(_sequence_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.seq_sequence
    ADD CONSTRAINT seq_sequence__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_sequence
    ADD CONSTRAINT seq_sequence__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_sequence
    ADD CONSTRAINT seq_sequence__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_sequence
    ADD CONSTRAINT seq_sequence__sequenceprovider_key_fkey FOREIGN KEY (_sequenceprovider_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_sequence
    ADD CONSTRAINT seq_sequence__sequencequality_key_fkey FOREIGN KEY (_sequencequality_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_sequence
    ADD CONSTRAINT seq_sequence__sequencestatus_key_fkey FOREIGN KEY (_sequencestatus_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_sequence
    ADD CONSTRAINT seq_sequence__sequencetype_key_fkey FOREIGN KEY (_sequencetype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_sequence_assoc
    ADD CONSTRAINT seq_sequence_assoc__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_sequence_assoc
    ADD CONSTRAINT seq_sequence_assoc__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_sequence_assoc
    ADD CONSTRAINT seq_sequence_assoc__qualifier_key_fkey FOREIGN KEY (_qualifier_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_sequence_assoc
    ADD CONSTRAINT seq_sequence_assoc__sequence_key_1_fkey FOREIGN KEY (_sequence_key_1) REFERENCES mgd.seq_sequence(_sequence_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.seq_sequence_assoc
    ADD CONSTRAINT seq_sequence_assoc__sequence_key_2_fkey FOREIGN KEY (_sequence_key_2) REFERENCES mgd.seq_sequence(_sequence_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.seq_sequence_raw
    ADD CONSTRAINT seq_sequence_raw__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_sequence_raw
    ADD CONSTRAINT seq_sequence_raw__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_sequence_raw
    ADD CONSTRAINT seq_sequence_raw__sequence_key_fkey FOREIGN KEY (_sequence_key) REFERENCES mgd.seq_sequence(_sequence_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.seq_source_assoc
    ADD CONSTRAINT seq_source_assoc__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_source_assoc
    ADD CONSTRAINT seq_source_assoc__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.seq_source_assoc
    ADD CONSTRAINT seq_source_assoc__sequence_key_fkey FOREIGN KEY (_sequence_key) REFERENCES mgd.seq_sequence(_sequence_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.seq_source_assoc
    ADD CONSTRAINT seq_source_assoc__source_key_fkey FOREIGN KEY (_source_key) REFERENCES mgd.prb_source(_source_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_allele_cache
    ADD CONSTRAINT voc_allele_cache__allele_key_fkey FOREIGN KEY (_allele_key) REFERENCES mgd.all_allele(_allele_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_allele_cache
    ADD CONSTRAINT voc_allele_cache__term_key_fkey FOREIGN KEY (_term_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_annot
    ADD CONSTRAINT voc_annot__annottype_key_fkey FOREIGN KEY (_annottype_key) REFERENCES mgd.voc_annottype(_annottype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_annot
    ADD CONSTRAINT voc_annot__qualifier_key_fkey FOREIGN KEY (_qualifier_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_annot
    ADD CONSTRAINT voc_annot__term_key_fkey FOREIGN KEY (_term_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_annot_count_cache
    ADD CONSTRAINT voc_annot_count_cache__term_key_fkey FOREIGN KEY (_term_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_annotheader
    ADD CONSTRAINT voc_annotheader__annottype_key_fkey FOREIGN KEY (_annottype_key) REFERENCES mgd.voc_annottype(_annottype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_annotheader
    ADD CONSTRAINT voc_annotheader__approvedby_key_fkey FOREIGN KEY (_approvedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_annotheader
    ADD CONSTRAINT voc_annotheader__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_annotheader
    ADD CONSTRAINT voc_annotheader__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_annotheader
    ADD CONSTRAINT voc_annotheader__term_key_fkey FOREIGN KEY (_term_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_annottype
    ADD CONSTRAINT voc_annottype__evidencevocab_key_fkey FOREIGN KEY (_evidencevocab_key) REFERENCES mgd.voc_vocab(_vocab_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_annottype
    ADD CONSTRAINT voc_annottype__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_annottype
    ADD CONSTRAINT voc_annottype__qualifiervocab_key_fkey FOREIGN KEY (_qualifiervocab_key) REFERENCES mgd.voc_vocab(_vocab_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_annottype
    ADD CONSTRAINT voc_annottype__vocab_key_fkey FOREIGN KEY (_vocab_key) REFERENCES mgd.voc_vocab(_vocab_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.voc_evidence
    ADD CONSTRAINT voc_evidence__annot_key_fkey FOREIGN KEY (_annot_key) REFERENCES mgd.voc_annot(_annot_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.voc_evidence
    ADD CONSTRAINT voc_evidence__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_evidence
    ADD CONSTRAINT voc_evidence__evidenceterm_key_fkey FOREIGN KEY (_evidenceterm_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_evidence
    ADD CONSTRAINT voc_evidence__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_evidence
    ADD CONSTRAINT voc_evidence__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_evidence_property
    ADD CONSTRAINT voc_evidence_property__annotevidence_key_fkey FOREIGN KEY (_annotevidence_key) REFERENCES mgd.voc_evidence(_annotevidence_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.voc_evidence_property
    ADD CONSTRAINT voc_evidence_property__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_evidence_property
    ADD CONSTRAINT voc_evidence_property__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_evidence_property
    ADD CONSTRAINT voc_evidence_property__propertyterm_key_fkey FOREIGN KEY (_propertyterm_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_marker_cache
    ADD CONSTRAINT voc_marker_cache__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.voc_marker_cache
    ADD CONSTRAINT voc_marker_cache__term_key_fkey FOREIGN KEY (_term_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_term
    ADD CONSTRAINT voc_term__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_term
    ADD CONSTRAINT voc_term__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_term
    ADD CONSTRAINT voc_term__vocab_key_fkey FOREIGN KEY (_vocab_key) REFERENCES mgd.voc_vocab(_vocab_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_term_emapa
    ADD CONSTRAINT voc_term_emapa__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_term_emapa
    ADD CONSTRAINT voc_term_emapa__defaultparent_key_fkey FOREIGN KEY (_defaultparent_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.voc_term_emapa
    ADD CONSTRAINT voc_term_emapa__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_term_emapa
    ADD CONSTRAINT voc_term_emapa__term_key_fkey FOREIGN KEY (_term_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.voc_term_emapa
    ADD CONSTRAINT voc_term_emapa_endstage_fkey FOREIGN KEY (endstage) REFERENCES mgd.gxd_theilerstage(_stage_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_term_emapa
    ADD CONSTRAINT voc_term_emapa_startstage_fkey FOREIGN KEY (startstage) REFERENCES mgd.gxd_theilerstage(_stage_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_term_emaps
    ADD CONSTRAINT voc_term_emaps__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_term_emaps
    ADD CONSTRAINT voc_term_emaps__defaultparent_key_fkey FOREIGN KEY (_defaultparent_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.voc_term_emaps
    ADD CONSTRAINT voc_term_emaps__emapa_term_key_fkey FOREIGN KEY (_emapa_term_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.voc_term_emaps
    ADD CONSTRAINT voc_term_emaps__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_term_emaps
    ADD CONSTRAINT voc_term_emaps__stage_key_fkey FOREIGN KEY (_stage_key) REFERENCES mgd.gxd_theilerstage(_stage_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_term_emaps
    ADD CONSTRAINT voc_term_emaps__term_key_fkey FOREIGN KEY (_term_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.voc_vocab
    ADD CONSTRAINT voc_vocab__logicaldb_key_fkey FOREIGN KEY (_logicaldb_key) REFERENCES mgd.acc_logicaldb(_logicaldb_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_vocab
    ADD CONSTRAINT voc_vocab__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


ALTER TABLE ONLY mgd.voc_vocabdag
    ADD CONSTRAINT voc_vocabdag__dag_key_fkey FOREIGN KEY (_dag_key) REFERENCES mgd.dag_dag(_dag_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.voc_vocabdag
    ADD CONSTRAINT voc_vocabdag__vocab_key_fkey FOREIGN KEY (_vocab_key) REFERENCES mgd.voc_vocab(_vocab_key) ON DELETE CASCADE DEFERRABLE;


ALTER TABLE ONLY mgd.wks_rosetta
    ADD CONSTRAINT wks_rosetta__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;



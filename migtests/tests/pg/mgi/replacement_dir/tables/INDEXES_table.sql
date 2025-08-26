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

CREATE INDEX acc_accession_0 ON mgd.acc_accession USING btree (lower(accid) ASC);

CREATE INDEX acc_accession_1 ON mgd.acc_accession USING btree (lower(prefixpart) ASC);

CREATE INDEX acc_accession_idx_accid ON mgd.acc_accession USING btree (accid ASC);

CREATE INDEX acc_accession_idx_clustered ON mgd.acc_accession USING btree (_object_key ASC, _mgitype_key);

CREATE INDEX acc_accession_idx_createdby_key ON mgd.acc_accession USING btree (_createdby_key ASC);

CREATE INDEX acc_accession_idx_creation_date ON mgd.acc_accession USING btree (creation_date ASC);

CREATE INDEX acc_accession_idx_logicaldb_key ON mgd.acc_accession USING btree (_logicaldb_key ASC);

CREATE INDEX acc_accession_idx_mgitype_key ON mgd.acc_accession USING btree (_mgitype_key ASC);

CREATE INDEX acc_accession_idx_modification_date ON mgd.acc_accession USING btree (modification_date ASC);

CREATE INDEX acc_accession_idx_modifiedby_key ON mgd.acc_accession USING btree (_modifiedby_key ASC);

CREATE INDEX acc_accession_idx_numericpart ON mgd.acc_accession USING btree (numericpart ASC);

CREATE INDEX acc_accession_idx_prefixpart ON mgd.acc_accession USING btree (prefixpart ASC);

CREATE INDEX acc_accessionreference_idx_createdby_key ON mgd.acc_accessionreference USING btree (_createdby_key ASC);

CREATE INDEX acc_accessionreference_idx_creation_date ON mgd.acc_accessionreference USING btree (creation_date ASC);

CREATE INDEX acc_accessionreference_idx_modification_date ON mgd.acc_accessionreference USING btree (modification_date ASC);

CREATE INDEX acc_accessionreference_idx_modifiedby_key ON mgd.acc_accessionreference USING btree (_modifiedby_key ASC);

CREATE INDEX acc_accessionreference_idx_refs_key ON mgd.acc_accessionreference USING btree (_refs_key ASC);

CREATE INDEX acc_actualdb_idx_logicaldb_key ON mgd.acc_actualdb USING btree (_logicaldb_key ASC);

CREATE INDEX acc_actualdb_idx_name ON mgd.acc_actualdb USING btree (name ASC);

CREATE UNIQUE INDEX acc_logicaldb_idx_name ON mgd.acc_logicaldb USING btree (name ASC, _logicaldb_key);

CREATE INDEX acc_logicaldb_idx_organism_key ON mgd.acc_logicaldb USING btree (_organism_key ASC);

CREATE INDEX acc_mgitype_0 ON mgd.acc_mgitype USING btree (lower(name) ASC);

CREATE UNIQUE INDEX acc_mgitype_idx_name ON mgd.acc_mgitype USING btree (name ASC);

CREATE INDEX all_allele_cellline_idx_allele_key ON mgd.all_allele_cellline USING btree (_allele_key ASC, _mutantcellline_key);

CREATE INDEX all_allele_cellline_idx_createdby_key ON mgd.all_allele_cellline USING btree (_createdby_key ASC);

CREATE INDEX all_allele_cellline_idx_creation_date ON mgd.all_allele_cellline USING btree (creation_date ASC);

CREATE INDEX all_allele_cellline_idx_modification_date ON mgd.all_allele_cellline USING btree (modification_date ASC);

CREATE INDEX all_allele_cellline_idx_modifiedby_key ON mgd.all_allele_cellline USING btree (_modifiedby_key ASC);

CREATE INDEX all_allele_cellline_idx_mutantcellline_key ON mgd.all_allele_cellline USING btree (_mutantcellline_key ASC);

CREATE INDEX all_allele_idx_allele_status_key ON mgd.all_allele USING btree (_allele_status_key ASC);

CREATE INDEX all_allele_idx_allele_type_key ON mgd.all_allele USING btree (_allele_type_key ASC);

CREATE INDEX all_allele_idx_clustered ON mgd.all_allele USING btree (_marker_key ASC);

CREATE INDEX all_allele_idx_collection_key ON mgd.all_allele USING btree (_collection_key ASC);

CREATE INDEX all_allele_idx_createdby_key ON mgd.all_allele USING btree (_createdby_key ASC);

CREATE INDEX all_allele_idx_creation_date ON mgd.all_allele USING btree (creation_date ASC);

CREATE INDEX all_allele_idx_markerallele_status_key ON mgd.all_allele USING btree (_markerallele_status_key ASC);

CREATE INDEX all_allele_idx_mode_key ON mgd.all_allele USING btree (_mode_key ASC);

CREATE INDEX all_allele_idx_modification_date ON mgd.all_allele USING btree (modification_date ASC);

CREATE INDEX all_allele_idx_modifiedby_key ON mgd.all_allele USING btree (_modifiedby_key ASC);

CREATE INDEX all_allele_idx_name ON mgd.all_allele USING btree (name ASC);

CREATE INDEX all_allele_idx_strain_key ON mgd.all_allele USING btree (_strain_key ASC);

CREATE INDEX all_allele_idx_symbol ON mgd.all_allele USING btree (symbol ASC);

CREATE INDEX all_allele_idx_transmission_key ON mgd.all_allele USING btree (_transmission_key ASC);

CREATE INDEX all_allele_mutation_idx_allele_key ON mgd.all_allele_mutation USING btree (_allele_key ASC);

CREATE INDEX all_allele_mutation_idx_creation_date ON mgd.all_allele_mutation USING btree (creation_date ASC);

CREATE INDEX all_allele_mutation_idx_modification_date ON mgd.all_allele_mutation USING btree (modification_date ASC);

CREATE INDEX all_allele_mutation_idx_mutation_key ON mgd.all_allele_mutation USING btree (_mutation_key ASC);

CREATE INDEX all_cellline_derivation_idx_createdby_key ON mgd.all_cellline_derivation USING btree (_createdby_key ASC);

CREATE INDEX all_cellline_derivation_idx_creation_date ON mgd.all_cellline_derivation USING btree (creation_date ASC);

CREATE INDEX all_cellline_derivation_idx_creator_key ON mgd.all_cellline_derivation USING btree (_creator_key ASC);

CREATE INDEX all_cellline_derivation_idx_derivationtype_key ON mgd.all_cellline_derivation USING btree (_derivationtype_key ASC, _parentcellline_key, _creator_key);

CREATE INDEX all_cellline_derivation_idx_modification_date ON mgd.all_cellline_derivation USING btree (modification_date ASC);

CREATE INDEX all_cellline_derivation_idx_modifiedby_key ON mgd.all_cellline_derivation USING btree (_modifiedby_key ASC);

CREATE INDEX all_cellline_derivation_idx_name ON mgd.all_cellline_derivation USING btree (name ASC, _derivation_key);

CREATE INDEX all_cellline_derivation_idx_pcl ON mgd.all_cellline_derivation USING btree (_parentcellline_key ASC, _derivation_key);

CREATE INDEX all_cellline_derivation_idx_refs_key ON mgd.all_cellline_derivation USING btree (_refs_key ASC);

CREATE INDEX all_cellline_derivation_idx_vector_key ON mgd.all_cellline_derivation USING btree (_vector_key ASC);

CREATE INDEX all_cellline_derivation_idx_vectortype_key ON mgd.all_cellline_derivation USING btree (_vectortype_key ASC);

CREATE INDEX all_cellline_idx_cellline ON mgd.all_cellline USING btree (cellline ASC);

CREATE INDEX all_cellline_idx_cellline_type_key ON mgd.all_cellline USING btree (_cellline_type_key ASC, _derivation_key, _cellline_key);

CREATE INDEX all_cellline_idx_createdby_key ON mgd.all_cellline USING btree (_createdby_key ASC);

CREATE INDEX all_cellline_idx_creation_date ON mgd.all_cellline USING btree (creation_date ASC);

CREATE INDEX all_cellline_idx_derivation_key ON mgd.all_cellline USING btree (_derivation_key ASC, _cellline_key);

CREATE INDEX all_cellline_idx_modification_date ON mgd.all_cellline USING btree (modification_date ASC);

CREATE INDEX all_cellline_idx_modifiedby_key ON mgd.all_cellline USING btree (_modifiedby_key ASC);

CREATE INDEX all_cellline_idx_strain_key ON mgd.all_cellline USING btree (_strain_key ASC);

CREATE INDEX all_cre_cache_idx_allele_type_key ON mgd.all_cre_cache USING btree (_allele_type_key ASC);

CREATE INDEX all_cre_cache_idx_assay_key ON mgd.all_cre_cache USING btree (_assay_key ASC);

CREATE INDEX all_cre_cache_idx_celltype_term_key ON mgd.all_cre_cache USING btree (_celltype_term_key ASC);

CREATE INDEX all_cre_cache_idx_clustered ON mgd.all_cre_cache USING btree (_allele_key ASC, _emapa_term_key, _stage_key, _assay_key, expressed);

CREATE INDEX all_cre_cache_idx_emapa_term_key ON mgd.all_cre_cache USING btree (_emapa_term_key ASC);

CREATE INDEX all_cre_cache_idx_stage_key ON mgd.all_cre_cache USING btree (_stage_key ASC);

CREATE INDEX all_knockout_cache_idx_clustered ON mgd.all_knockout_cache USING btree (_allele_key ASC);

CREATE UNIQUE INDEX all_knockout_cache_idx_marker_key ON mgd.all_knockout_cache USING btree (_marker_key ASC);

CREATE INDEX all_label_0 ON mgd.all_label USING btree (lower(label) ASC);

CREATE INDEX all_label_idx_label ON mgd.all_label USING btree (label ASC);

CREATE INDEX all_label_idx_label_status_key ON mgd.all_label USING btree (_label_status_key ASC);

CREATE INDEX all_label_idx_priority ON mgd.all_label USING btree (priority ASC);

CREATE INDEX all_variant_idx_allele_key ON mgd.all_variant USING btree (_allele_key ASC);

CREATE INDEX all_variant_idx_createdby_key ON mgd.all_variant USING btree (_createdby_key ASC);

CREATE INDEX all_variant_idx_creation_date ON mgd.all_variant USING btree (creation_date ASC);

CREATE INDEX all_variant_idx_modification_date ON mgd.all_variant USING btree (modification_date ASC);

CREATE INDEX all_variant_idx_modifiedby_key ON mgd.all_variant USING btree (_modifiedby_key ASC);

CREATE INDEX all_variant_idx_sourcevariant_key ON mgd.all_variant USING btree (_sourcevariant_key ASC);

CREATE INDEX all_variant_idx_strain_key ON mgd.all_variant USING btree (_strain_key ASC);

CREATE INDEX all_variant_sequence_idx_createdby_key ON mgd.all_variant_sequence USING btree (_createdby_key ASC);

CREATE INDEX all_variant_sequence_idx_creation_date ON mgd.all_variant_sequence USING btree (creation_date ASC);

CREATE INDEX all_variant_sequence_idx_modification_date ON mgd.all_variant_sequence USING btree (modification_date ASC);

CREATE INDEX all_variant_sequence_idx_modifiedby_key ON mgd.all_variant_sequence USING btree (_modifiedby_key ASC);

CREATE INDEX all_variant_sequence_idx_sequence_type_key ON mgd.all_variant_sequence USING btree (_sequence_type_key ASC);

CREATE INDEX all_variant_sequence_idx_variant_key ON mgd.all_variant_sequence USING btree (_variant_key ASC);

CREATE INDEX bib_books_idx_creation_date ON mgd.bib_books USING btree (creation_date ASC);

CREATE INDEX bib_books_idx_modification_date ON mgd.bib_books USING btree (modification_date ASC);

CREATE INDEX bib_citation_cache_idx_doiid ON mgd.bib_citation_cache USING btree (doiid ASC);

CREATE INDEX bib_citation_cache_idx_jnumid ON mgd.bib_citation_cache USING btree (jnumid ASC);

CREATE INDEX bib_citation_cache_idx_journal ON mgd.bib_citation_cache USING btree (journal ASC);

CREATE INDEX bib_citation_cache_idx_mgiid ON mgd.bib_citation_cache USING btree (mgiid ASC);

CREATE INDEX bib_citation_cache_idx_numericpart ON mgd.bib_citation_cache USING btree (numericpart ASC);

CREATE INDEX bib_citation_cache_idx_pubmedid ON mgd.bib_citation_cache USING btree (pubmedid ASC);

CREATE INDEX bib_citation_cache_idx_relevance_key ON mgd.bib_citation_cache USING btree (_relevance_key ASC);

CREATE INDEX bib_notes_idx_creation_date ON mgd.bib_notes USING btree (creation_date ASC);

CREATE INDEX bib_notes_idx_modification_date ON mgd.bib_notes USING btree (modification_date ASC);

CREATE INDEX bib_refs_idx_authors ON mgd.bib_refs USING btree ((md5(authors)::uuid) ASC);

CREATE INDEX bib_refs_idx_createdby_key ON mgd.bib_refs USING btree (_createdby_key ASC);

CREATE INDEX bib_refs_idx_creation_date ON mgd.bib_refs USING btree (creation_date ASC);

CREATE INDEX bib_refs_idx_isprimary ON mgd.bib_refs USING btree (_primary ASC);

CREATE INDEX bib_refs_idx_journal ON mgd.bib_refs USING btree (journal ASC);

CREATE INDEX bib_refs_idx_modification_date ON mgd.bib_refs USING btree (modification_date ASC);

CREATE INDEX bib_refs_idx_modifiedby_key ON mgd.bib_refs USING btree (_modifiedby_key ASC);

CREATE INDEX bib_refs_idx_referencetype_key ON mgd.bib_refs USING btree (_referencetype_key ASC);

CREATE INDEX bib_refs_idx_title ON mgd.bib_refs USING btree (title ASC);

CREATE INDEX bib_refs_idx_year ON mgd.bib_refs USING btree (year ASC);

CREATE INDEX bib_workflow_data_idx_createdby_key ON mgd.bib_workflow_data USING btree (_createdby_key ASC);

CREATE INDEX bib_workflow_data_idx_extractedtext_key ON mgd.bib_workflow_data USING btree (_extractedtext_key ASC);

CREATE INDEX bib_workflow_data_idx_haspdf ON mgd.bib_workflow_data USING btree (haspdf ASC);

CREATE INDEX bib_workflow_data_idx_modifiedby_key ON mgd.bib_workflow_data USING btree (_modifiedby_key ASC);

CREATE INDEX bib_workflow_data_idx_refs_key ON mgd.bib_workflow_data USING btree (_refs_key ASC);

CREATE INDEX bib_workflow_data_idx_supplemental_key ON mgd.bib_workflow_data USING btree (_supplemental_key ASC);

CREATE INDEX bib_workflow_relevance_idx_confidence ON mgd.bib_workflow_relevance USING btree (confidence ASC);

CREATE INDEX bib_workflow_relevance_idx_createdby_key ON mgd.bib_workflow_relevance USING btree (_createdby_key ASC);

CREATE INDEX bib_workflow_relevance_idx_creation_date ON mgd.bib_workflow_relevance USING btree (creation_date ASC);

CREATE INDEX bib_workflow_relevance_idx_iscurrent ON mgd.bib_workflow_relevance USING btree (iscurrent ASC);

CREATE INDEX bib_workflow_relevance_idx_modification_date ON mgd.bib_workflow_relevance USING btree (modification_date ASC);

CREATE INDEX bib_workflow_relevance_idx_modifiedby_key ON mgd.bib_workflow_relevance USING btree (_modifiedby_key ASC);

CREATE INDEX bib_workflow_relevance_idx_refs_key ON mgd.bib_workflow_relevance USING btree (_refs_key ASC);

CREATE INDEX bib_workflow_relevance_idx_relevance_key ON mgd.bib_workflow_relevance USING btree (_relevance_key ASC);

CREATE INDEX bib_workflow_status_idx_createdby_key ON mgd.bib_workflow_status USING btree (_createdby_key ASC);

CREATE INDEX bib_workflow_status_idx_creation_date ON mgd.bib_workflow_status USING btree (creation_date ASC);

CREATE INDEX bib_workflow_status_idx_group_key ON mgd.bib_workflow_status USING btree (_group_key ASC);

CREATE INDEX bib_workflow_status_idx_iscurrent ON mgd.bib_workflow_status USING btree (iscurrent ASC);

CREATE INDEX bib_workflow_status_idx_modification_date ON mgd.bib_workflow_status USING btree (modification_date ASC);

CREATE INDEX bib_workflow_status_idx_modifiedby_key ON mgd.bib_workflow_status USING btree (_modifiedby_key ASC);

CREATE INDEX bib_workflow_status_idx_refs_key ON mgd.bib_workflow_status USING btree (_refs_key ASC);

CREATE INDEX bib_workflow_status_idx_status_key ON mgd.bib_workflow_status USING btree (_status_key ASC);

CREATE INDEX bib_workflow_tag_idx_createdby_key ON mgd.bib_workflow_tag USING btree (_createdby_key ASC);

CREATE INDEX bib_workflow_tag_idx_creation_date ON mgd.bib_workflow_tag USING btree (creation_date ASC);

CREATE INDEX bib_workflow_tag_idx_modification_date ON mgd.bib_workflow_tag USING btree (modification_date ASC);

CREATE INDEX bib_workflow_tag_idx_modifiedby_key ON mgd.bib_workflow_tag USING btree (_modifiedby_key ASC);

CREATE INDEX bib_workflow_tag_idx_refs_key ON mgd.bib_workflow_tag USING btree (_refs_key ASC);

CREATE INDEX bib_workflow_tag_idx_tag_key ON mgd.bib_workflow_tag USING btree (_tag_key ASC);

CREATE INDEX crs_cross_idx_femalestrain_key ON mgd.crs_cross USING btree (_femalestrain_key ASC);

CREATE INDEX crs_cross_idx_malestrain_key ON mgd.crs_cross USING btree (_malestrain_key ASC);

CREATE INDEX crs_cross_idx_strainho_key ON mgd.crs_cross USING btree (_strainho_key ASC);

CREATE INDEX crs_cross_idx_strainht_key ON mgd.crs_cross USING btree (_strainht_key ASC);

CREATE INDEX crs_cross_idx_type ON mgd.crs_cross USING btree (type ASC);

CREATE INDEX crs_cross_idx_whosecross ON mgd.crs_cross USING btree (whosecross ASC);

CREATE UNIQUE INDEX crs_matrix_idx_clustered ON mgd.crs_matrix USING btree (_cross_key ASC, _marker_key, othersymbol, chromosome, rownumber);

CREATE INDEX crs_matrix_idx_marker_key ON mgd.crs_matrix USING btree (_marker_key ASC);

CREATE INDEX crs_references_idx_marker_key ON mgd.crs_references USING btree (_marker_key ASC);

CREATE INDEX dag_closure_idx_ancestor_key ON mgd.dag_closure USING btree (_ancestor_key ASC);

CREATE INDEX dag_closure_idx_ancestorlabel_key ON mgd.dag_closure USING btree (_ancestorlabel_key ASC);

CREATE INDEX dag_closure_idx_clustered ON mgd.dag_closure USING btree (_ancestorobject_key ASC, _descendentobject_key, _dag_key);

CREATE INDEX dag_closure_idx_descendent_key ON mgd.dag_closure USING btree (_descendent_key ASC);

CREATE INDEX dag_closure_idx_descendentlabel_key ON mgd.dag_closure USING btree (_descendentlabel_key ASC);

CREATE INDEX dag_closure_idx_descendentobject_key ON mgd.dag_closure USING btree (_descendentobject_key ASC, _ancestorobject_key, _dag_key);

CREATE INDEX dag_closure_idx_mgitype_key ON mgd.dag_closure USING btree (_mgitype_key ASC);

CREATE INDEX dag_dag_idx_mgitype_key ON mgd.dag_dag USING btree (_mgitype_key ASC);

CREATE INDEX dag_dag_idx_refs_key ON mgd.dag_dag USING btree (_refs_key ASC);

CREATE INDEX dag_edge_idx_child_key ON mgd.dag_edge USING btree (_child_key ASC);

CREATE INDEX dag_edge_idx_clustered ON mgd.dag_edge USING btree (_parent_key ASC, _child_key, _label_key);

CREATE INDEX dag_edge_idx_dag_key ON mgd.dag_edge USING btree (_dag_key ASC);

CREATE INDEX dag_edge_idx_label_key ON mgd.dag_edge USING btree (_label_key ASC);

CREATE INDEX dag_label_idx_label ON mgd.dag_label USING btree (label ASC);

CREATE INDEX dag_node_idx_clustered ON mgd.dag_node USING btree (_dag_key ASC, _object_key);

CREATE INDEX dag_node_idx_label_key ON mgd.dag_node USING btree (_label_key ASC);

CREATE INDEX dag_node_idx_object_key ON mgd.dag_node USING btree (_object_key ASC);

CREATE INDEX go_tracking_idx_completedby_key ON mgd.go_tracking USING btree (_completedby_key ASC);

CREATE INDEX go_tracking_idx_completion_date ON mgd.go_tracking USING btree (completion_date ASC);

CREATE INDEX go_tracking_idx_createdby_key ON mgd.go_tracking USING btree (_createdby_key ASC);

CREATE INDEX go_tracking_idx_creation_date ON mgd.go_tracking USING btree (creation_date ASC);

CREATE INDEX go_tracking_idx_modification_date ON mgd.go_tracking USING btree (modification_date ASC);

CREATE INDEX go_tracking_idx_modifiedby_key ON mgd.go_tracking USING btree (_modifiedby_key ASC);

CREATE INDEX gxd_allelegenotype_idx_allele_key ON mgd.gxd_allelegenotype USING btree (_allele_key ASC, _genotype_key);

CREATE INDEX gxd_allelegenotype_idx_createdby_key ON mgd.gxd_allelegenotype USING btree (_createdby_key ASC);

CREATE INDEX gxd_allelegenotype_idx_marker_key ON mgd.gxd_allelegenotype USING btree (_marker_key ASC);

CREATE INDEX gxd_allelegenotype_idx_modifiedby_key ON mgd.gxd_allelegenotype USING btree (_modifiedby_key ASC);

CREATE INDEX gxd_allelepair_idx_allele_key_2 ON mgd.gxd_allelepair USING btree (_allele_key_2 ASC);

CREATE INDEX gxd_allelepair_idx_clustered ON mgd.gxd_allelepair USING btree (_allele_key_1 ASC);

CREATE INDEX gxd_allelepair_idx_compound_key ON mgd.gxd_allelepair USING btree (_compound_key ASC);

CREATE INDEX gxd_allelepair_idx_createdby_key ON mgd.gxd_allelepair USING btree (_createdby_key ASC);

CREATE INDEX gxd_allelepair_idx_creation_date ON mgd.gxd_allelepair USING btree (creation_date ASC);

CREATE INDEX gxd_allelepair_idx_genotype_key ON mgd.gxd_allelepair USING btree (_genotype_key ASC);

CREATE INDEX gxd_allelepair_idx_marker_key ON mgd.gxd_allelepair USING btree (_marker_key ASC);

CREATE INDEX gxd_allelepair_idx_modification_date ON mgd.gxd_allelepair USING btree (modification_date ASC);

CREATE INDEX gxd_allelepair_idx_modifiedby_key ON mgd.gxd_allelepair USING btree (_modifiedby_key ASC);

CREATE INDEX gxd_allelepair_idx_mutantcellline_key_1 ON mgd.gxd_allelepair USING btree (_mutantcellline_key_1 ASC);

CREATE INDEX gxd_allelepair_idx_mutantcellline_key_2 ON mgd.gxd_allelepair USING btree (_mutantcellline_key_2 ASC);

CREATE INDEX gxd_allelepair_idx_pairstate_key ON mgd.gxd_allelepair USING btree (_pairstate_key ASC);

CREATE INDEX gxd_antibody_idx_antibodyclass_key ON mgd.gxd_antibody USING btree (_antibodyclass_key ASC);

CREATE INDEX gxd_antibody_idx_antibodytype_key ON mgd.gxd_antibody USING btree (_antibodytype_key ASC);

CREATE INDEX gxd_antibody_idx_antigen_key ON mgd.gxd_antibody USING btree (_antigen_key ASC);

CREATE INDEX gxd_antibody_idx_createdby_key ON mgd.gxd_antibody USING btree (_createdby_key ASC);

CREATE INDEX gxd_antibody_idx_creation_date ON mgd.gxd_antibody USING btree (creation_date ASC);

CREATE INDEX gxd_antibody_idx_modification_date ON mgd.gxd_antibody USING btree (modification_date ASC);

CREATE INDEX gxd_antibody_idx_modifiedby_key ON mgd.gxd_antibody USING btree (_modifiedby_key ASC);

CREATE INDEX gxd_antibody_idx_organism_key ON mgd.gxd_antibody USING btree (_organism_key ASC);

CREATE INDEX gxd_antibodyalias_idx_clustered ON mgd.gxd_antibodyalias USING btree (_antibody_key ASC);

CREATE INDEX gxd_antibodyalias_idx_refs_key ON mgd.gxd_antibodyalias USING btree (_refs_key ASC);

CREATE INDEX gxd_antibodymarker_idx_antibody_key ON mgd.gxd_antibodymarker USING btree (_antibody_key ASC);

CREATE INDEX gxd_antibodymarker_idx_marker_key ON mgd.gxd_antibodymarker USING btree (_marker_key ASC);

CREATE INDEX gxd_antibodyprep_idx_antibody_key ON mgd.gxd_antibodyprep USING btree (_antibody_key ASC);

CREATE INDEX gxd_antibodyprep_idx_label_key ON mgd.gxd_antibodyprep USING btree (_label_key ASC);

CREATE INDEX gxd_antibodyprep_idx_secondary_key ON mgd.gxd_antibodyprep USING btree (_secondary_key ASC);

CREATE INDEX gxd_antigen_idx_createdby_key ON mgd.gxd_antigen USING btree (_createdby_key ASC);

CREATE INDEX gxd_antigen_idx_creation_date ON mgd.gxd_antigen USING btree (creation_date ASC);

CREATE INDEX gxd_antigen_idx_modification_date ON mgd.gxd_antigen USING btree (modification_date ASC);

CREATE INDEX gxd_antigen_idx_modifiedby_key ON mgd.gxd_antigen USING btree (_modifiedby_key ASC);

CREATE INDEX gxd_antigen_idx_source_key ON mgd.gxd_antigen USING btree (_source_key ASC);

CREATE INDEX gxd_assay_idx_antibodyprep_key ON mgd.gxd_assay USING btree (_antibodyprep_key ASC);

CREATE INDEX gxd_assay_idx_assaytype_key ON mgd.gxd_assay USING btree (_assaytype_key ASC);

CREATE INDEX gxd_assay_idx_clustered ON mgd.gxd_assay USING btree (_marker_key ASC);

CREATE INDEX gxd_assay_idx_createdby_key ON mgd.gxd_assay USING btree (_createdby_key ASC);

CREATE INDEX gxd_assay_idx_creation_date ON mgd.gxd_assay USING btree (creation_date ASC);

CREATE INDEX gxd_assay_idx_imagepane_key ON mgd.gxd_assay USING btree (_imagepane_key ASC);

CREATE INDEX gxd_assay_idx_modification_date ON mgd.gxd_assay USING btree (modification_date ASC);

CREATE INDEX gxd_assay_idx_modifiedby_key ON mgd.gxd_assay USING btree (_modifiedby_key ASC);

CREATE INDEX gxd_assay_idx_probeprep_key ON mgd.gxd_assay USING btree (_probeprep_key ASC);

CREATE INDEX gxd_assay_idx_refs_key ON mgd.gxd_assay USING btree (_refs_key ASC);

CREATE INDEX gxd_assay_idx_reportergene_key ON mgd.gxd_assay USING btree (_reportergene_key ASC);

CREATE INDEX gxd_assaynote_idx_assay_key ON mgd.gxd_assaynote USING btree (_assay_key ASC);

CREATE INDEX gxd_assaynote_idx_assaynote ON mgd.gxd_assaynote USING btree (assaynote ASC);

CREATE INDEX gxd_expression_idx_assay_key ON mgd.gxd_expression USING btree (_assay_key ASC);

CREATE INDEX gxd_expression_idx_assaytype_key ON mgd.gxd_expression USING btree (_assaytype_key ASC);

CREATE INDEX gxd_expression_idx_celltype_term_key ON mgd.gxd_expression USING btree (_celltype_term_key ASC);

CREATE INDEX gxd_expression_idx_clustered ON mgd.gxd_expression USING btree (_marker_key ASC, _emapa_term_key, _stage_key, isforgxd);

CREATE INDEX gxd_expression_idx_emapa_term_etc_key ON mgd.gxd_expression USING btree (_emapa_term_key ASC, _stage_key, _expression_key, isforgxd);

CREATE INDEX gxd_expression_idx_gellane_key ON mgd.gxd_expression USING btree (_gellane_key ASC);

CREATE INDEX gxd_expression_idx_genotypegxd_key ON mgd.gxd_expression USING btree (_genotype_key ASC, isforgxd);

CREATE INDEX gxd_expression_idx_refs_etc_key ON mgd.gxd_expression USING btree (_refs_key ASC, _emapa_term_key, _stage_key, isforgxd);

CREATE INDEX gxd_expression_idx_specimen_key ON mgd.gxd_expression USING btree (_specimen_key ASC);

CREATE INDEX gxd_expression_idx_stage_key ON mgd.gxd_expression USING btree (_stage_key ASC);

CREATE INDEX gxd_gelband_idx_clustered ON mgd.gxd_gelband USING btree (_gellane_key ASC);

CREATE INDEX gxd_gelband_idx_gelrow_key ON mgd.gxd_gelband USING btree (_gelrow_key ASC);

CREATE INDEX gxd_gelband_idx_strength_key ON mgd.gxd_gelband USING btree (_strength_key ASC);

CREATE INDEX gxd_gellane_idx_clustered ON mgd.gxd_gellane USING btree (_assay_key ASC);

CREATE INDEX gxd_gellane_idx_gelcontrol_key ON mgd.gxd_gellane USING btree (_gelcontrol_key ASC);

CREATE INDEX gxd_gellane_idx_gelrnatype_key ON mgd.gxd_gellane USING btree (_gelrnatype_key ASC);

CREATE INDEX gxd_gellane_idx_genotype_key ON mgd.gxd_gellane USING btree (_genotype_key ASC);

CREATE INDEX gxd_gellanestructure_idx_emapa_term_key ON mgd.gxd_gellanestructure USING btree (_emapa_term_key ASC);

CREATE INDEX gxd_gellanestructure_idx_gellane_key ON mgd.gxd_gellanestructure USING btree (_gellane_key ASC);

CREATE INDEX gxd_gellanestructure_idx_stage_key ON mgd.gxd_gellanestructure USING btree (_stage_key ASC);

CREATE INDEX gxd_gelrow_idx_clustered ON mgd.gxd_gelrow USING btree (_assay_key ASC);

CREATE INDEX gxd_gelrow_idx_gelunits_key ON mgd.gxd_gelrow USING btree (_gelunits_key ASC);

CREATE INDEX gxd_genotype_idx_createdby_key ON mgd.gxd_genotype USING btree (_createdby_key ASC);

CREATE INDEX gxd_genotype_idx_creation_date ON mgd.gxd_genotype USING btree (creation_date ASC);

CREATE INDEX gxd_genotype_idx_existsas_key ON mgd.gxd_genotype USING btree (_existsas_key ASC);

CREATE INDEX gxd_genotype_idx_modification_date ON mgd.gxd_genotype USING btree (modification_date ASC);

CREATE INDEX gxd_genotype_idx_modifiedby_key ON mgd.gxd_genotype USING btree (_modifiedby_key ASC);

CREATE INDEX gxd_genotype_idx_strain_key ON mgd.gxd_genotype USING btree (_strain_key ASC);

CREATE INDEX gxd_htexperiment_idx_createdby_key ON mgd.gxd_htexperiment USING btree (_createdby_key ASC);

CREATE INDEX gxd_htexperiment_idx_creation_date ON mgd.gxd_htexperiment USING btree (creation_date ASC);

CREATE INDEX gxd_htexperiment_idx_curationstate_key ON mgd.gxd_htexperiment USING btree (_curationstate_key ASC);

CREATE INDEX gxd_htexperiment_idx_evaluatedby_key ON mgd.gxd_htexperiment USING btree (_evaluatedby_key ASC);

CREATE INDEX gxd_htexperiment_idx_evaluationstate_key ON mgd.gxd_htexperiment USING btree (_evaluationstate_key ASC);

CREATE INDEX gxd_htexperiment_idx_experimenttype_key ON mgd.gxd_htexperiment USING btree (_experimenttype_key ASC);

CREATE INDEX gxd_htexperiment_idx_initialcuratedby_key ON mgd.gxd_htexperiment USING btree (_initialcuratedby_key ASC);

CREATE INDEX gxd_htexperiment_idx_lastcuratedby_key ON mgd.gxd_htexperiment USING btree (_lastcuratedby_key ASC);

CREATE INDEX gxd_htexperiment_idx_modification_date ON mgd.gxd_htexperiment USING btree (modification_date ASC);

CREATE INDEX gxd_htexperiment_idx_modifiedby_key ON mgd.gxd_htexperiment USING btree (_modifiedby_key ASC);

CREATE INDEX gxd_htexperiment_idx_source_key ON mgd.gxd_htexperiment USING btree (_source_key ASC);

CREATE INDEX gxd_htexperiment_idx_studytype_key ON mgd.gxd_htexperiment USING btree (_studytype_key ASC);

CREATE INDEX gxd_htexperimentvariable_idx_experiment_key ON mgd.gxd_htexperimentvariable USING btree (_experiment_key ASC);

CREATE INDEX gxd_htexperimentvariable_idx_term_key ON mgd.gxd_htexperimentvariable USING btree (_term_key ASC);

CREATE INDEX gxd_htrawsample_idx_accid ON mgd.gxd_htrawsample USING btree (accid ASC);

CREATE INDEX gxd_htrawsample_idx_createdby_key ON mgd.gxd_htrawsample USING btree (_createdby_key ASC);

CREATE INDEX gxd_htrawsample_idx_creation_date ON mgd.gxd_htrawsample USING btree (creation_date ASC);

CREATE INDEX gxd_htrawsample_idx_modification_date ON mgd.gxd_htrawsample USING btree (modification_date ASC);

CREATE INDEX gxd_htrawsample_idx_modifiedby_key ON mgd.gxd_htrawsample USING btree (_modifiedby_key ASC);

CREATE INDEX gxd_htsample_idx_celltype_term_key ON mgd.gxd_htsample USING btree (_celltype_term_key ASC);

CREATE INDEX gxd_htsample_idx_createdby_key ON mgd.gxd_htsample USING btree (_createdby_key ASC);

CREATE INDEX gxd_htsample_idx_creation_date ON mgd.gxd_htsample USING btree (creation_date ASC);

CREATE INDEX gxd_htsample_idx_emapa_key ON mgd.gxd_htsample USING btree (_emapa_key ASC);

CREATE INDEX gxd_htsample_idx_experiment_key ON mgd.gxd_htsample USING btree (_experiment_key ASC);

CREATE INDEX gxd_htsample_idx_genotype_key ON mgd.gxd_htsample USING btree (_genotype_key ASC);

CREATE INDEX gxd_htsample_idx_modification_date ON mgd.gxd_htsample USING btree (modification_date ASC);

CREATE INDEX gxd_htsample_idx_modifiedby_key ON mgd.gxd_htsample USING btree (_modifiedby_key ASC);

CREATE INDEX gxd_htsample_idx_organism_key ON mgd.gxd_htsample USING btree (_organism_key ASC);

CREATE INDEX gxd_htsample_idx_relevance_key ON mgd.gxd_htsample USING btree (_relevance_key ASC);

CREATE INDEX gxd_htsample_idx_sex_key ON mgd.gxd_htsample USING btree (_sex_key ASC);

CREATE INDEX gxd_htsample_idx_stage_key ON mgd.gxd_htsample USING btree (_stage_key ASC);

CREATE INDEX gxd_htsample_rnaseq_idx_createdby_key ON mgd.gxd_htsample_rnaseq USING btree (_createdby_key ASC);

CREATE INDEX gxd_htsample_rnaseq_idx_marker_key ON mgd.gxd_htsample_rnaseq USING btree (_marker_key ASC);

CREATE INDEX gxd_htsample_rnaseq_idx_modifiedby_key ON mgd.gxd_htsample_rnaseq USING btree (_modifiedby_key ASC);

CREATE INDEX gxd_htsample_rnaseq_idx_rnaseqcombined_key ON mgd.gxd_htsample_rnaseq USING btree (_rnaseqcombined_key ASC);

CREATE INDEX gxd_htsample_rnaseq_idx_sample_key ON mgd.gxd_htsample_rnaseq USING btree (_sample_key ASC);

CREATE INDEX gxd_htsample_rnaseqcombined_idx_createdby_key ON mgd.gxd_htsample_rnaseqcombined USING btree (_createdby_key ASC);

CREATE INDEX gxd_htsample_rnaseqcombined_idx_level_key ON mgd.gxd_htsample_rnaseqcombined USING btree (_level_key ASC);

CREATE INDEX gxd_htsample_rnaseqcombined_idx_marker_key ON mgd.gxd_htsample_rnaseqcombined USING btree (_marker_key ASC);

CREATE INDEX gxd_htsample_rnaseqcombined_idx_modifiedby_key ON mgd.gxd_htsample_rnaseqcombined USING btree (_modifiedby_key ASC);

CREATE INDEX gxd_htsample_rnaseqset_cache_idx_createdby_key ON mgd.gxd_htsample_rnaseqset_cache USING btree (_createdby_key ASC);

CREATE INDEX gxd_htsample_rnaseqset_cache_idx_modifiedby_key ON mgd.gxd_htsample_rnaseqset_cache USING btree (_modifiedby_key ASC);

CREATE INDEX gxd_htsample_rnaseqset_cache_idx_rnaseqcombined_key ON mgd.gxd_htsample_rnaseqset_cache USING btree (_rnaseqcombined_key ASC);

CREATE INDEX gxd_htsample_rnaseqset_cache_idx_rnaseqset_key ON mgd.gxd_htsample_rnaseqset_cache USING btree (_rnaseqset_key ASC);

CREATE INDEX gxd_htsample_rnaseqset_idx_age ON mgd.gxd_htsample_rnaseqset USING btree (age ASC);

CREATE INDEX gxd_htsample_rnaseqset_idx_createdby_key ON mgd.gxd_htsample_rnaseqset USING btree (_createdby_key ASC);

CREATE INDEX gxd_htsample_rnaseqset_idx_emapa_key ON mgd.gxd_htsample_rnaseqset USING btree (_emapa_key ASC);

CREATE INDEX gxd_htsample_rnaseqset_idx_experiment_key ON mgd.gxd_htsample_rnaseqset USING btree (_experiment_key ASC);

CREATE INDEX gxd_htsample_rnaseqset_idx_genotype_key ON mgd.gxd_htsample_rnaseqset USING btree (_genotype_key ASC);

CREATE INDEX gxd_htsample_rnaseqset_idx_modifiedby_key ON mgd.gxd_htsample_rnaseqset USING btree (_modifiedby_key ASC);

CREATE INDEX gxd_htsample_rnaseqset_idx_note ON mgd.gxd_htsample_rnaseqset USING btree (note ASC);

CREATE INDEX gxd_htsample_rnaseqset_idx_organism_key ON mgd.gxd_htsample_rnaseqset USING btree (_organism_key ASC);

CREATE INDEX gxd_htsample_rnaseqset_idx_sex_key ON mgd.gxd_htsample_rnaseqset USING btree (_sex_key ASC);

CREATE INDEX gxd_htsample_rnaseqset_idx_stage_key ON mgd.gxd_htsample_rnaseqset USING btree (_stage_key ASC);

CREATE INDEX gxd_htsample_rnaseqsetmember_idx_createdby_key ON mgd.gxd_htsample_rnaseqsetmember USING btree (_createdby_key ASC);

CREATE INDEX gxd_htsample_rnaseqsetmember_idx_modifiedby_key ON mgd.gxd_htsample_rnaseqsetmember USING btree (_modifiedby_key ASC);

CREATE INDEX gxd_htsample_rnaseqsetmember_idx_rnaseqset_key ON mgd.gxd_htsample_rnaseqsetmember USING btree (_rnaseqset_key ASC);

CREATE INDEX gxd_htsample_rnaseqsetmember_idx_sample_key ON mgd.gxd_htsample_rnaseqsetmember USING btree (_sample_key ASC);

CREATE UNIQUE INDEX gxd_index_idx_clustered ON mgd.gxd_index USING btree (_marker_key ASC, _refs_key);

CREATE INDEX gxd_index_idx_conditionalmutants_key ON mgd.gxd_index USING btree (_conditionalmutants_key ASC);

CREATE INDEX gxd_index_idx_createdby_key ON mgd.gxd_index USING btree (_createdby_key ASC);

CREATE INDEX gxd_index_idx_creation_date ON mgd.gxd_index USING btree (creation_date ASC);

CREATE INDEX gxd_index_idx_modification_date ON mgd.gxd_index USING btree (modification_date ASC);

CREATE INDEX gxd_index_idx_modifiedby_key ON mgd.gxd_index USING btree (_modifiedby_key ASC);

CREATE INDEX gxd_index_idx_priority_key ON mgd.gxd_index USING btree (_priority_key ASC);

CREATE INDEX gxd_index_idx_refs_key ON mgd.gxd_index USING btree (_refs_key ASC);

CREATE INDEX gxd_index_stages_idx_createdby_key ON mgd.gxd_index_stages USING btree (_createdby_key ASC);

CREATE INDEX gxd_index_stages_idx_creation_date ON mgd.gxd_index_stages USING btree (creation_date ASC);

CREATE INDEX gxd_index_stages_idx_index_key ON mgd.gxd_index_stages USING btree (_index_key ASC);

CREATE INDEX gxd_index_stages_idx_indexassay_key ON mgd.gxd_index_stages USING btree (_indexassay_key ASC);

CREATE INDEX gxd_index_stages_idx_modification_date ON mgd.gxd_index_stages USING btree (modification_date ASC);

CREATE INDEX gxd_index_stages_idx_modifiedby_key ON mgd.gxd_index_stages USING btree (_modifiedby_key ASC);

CREATE INDEX gxd_index_stages_idx_stageid_key ON mgd.gxd_index_stages USING btree (_stageid_key ASC);

CREATE INDEX gxd_insituresult_idx_pattern_key ON mgd.gxd_insituresult USING btree (_pattern_key ASC);

CREATE INDEX gxd_insituresult_idx_specimen_key ON mgd.gxd_insituresult USING btree (_specimen_key ASC);

CREATE INDEX gxd_insituresult_idx_strength_key ON mgd.gxd_insituresult USING btree (_strength_key ASC);

CREATE INDEX gxd_insituresultimage_idx_imagepane_key ON mgd.gxd_insituresultimage USING btree (_imagepane_key ASC);

CREATE INDEX gxd_insituresultimage_idx_result_key ON mgd.gxd_insituresultimage USING btree (_result_key ASC);

CREATE INDEX gxd_isresultcelltype_idx_celltype_term_key ON mgd.gxd_isresultcelltype USING btree (_celltype_term_key ASC);

CREATE INDEX gxd_isresultcelltype_idx_result_key ON mgd.gxd_isresultcelltype USING btree (_result_key ASC);

CREATE INDEX gxd_isresultstructure_idx_emapa_term_key ON mgd.gxd_isresultstructure USING btree (_emapa_term_key ASC);

CREATE INDEX gxd_isresultstructure_idx_result_key ON mgd.gxd_isresultstructure USING btree (_result_key ASC);

CREATE INDEX gxd_isresultstructure_idx_stage_key ON mgd.gxd_isresultstructure USING btree (_stage_key ASC);

CREATE INDEX gxd_probeprep_idx_label_key ON mgd.gxd_probeprep USING btree (_label_key ASC);

CREATE INDEX gxd_probeprep_idx_probe_key ON mgd.gxd_probeprep USING btree (_probe_key ASC);

CREATE INDEX gxd_probeprep_idx_sense_key ON mgd.gxd_probeprep USING btree (_sense_key ASC);

CREATE INDEX gxd_probeprep_idx_visualization_key ON mgd.gxd_probeprep USING btree (_visualization_key ASC);

CREATE INDEX gxd_specimen_idx_clustered ON mgd.gxd_specimen USING btree (_assay_key ASC);

CREATE INDEX gxd_specimen_idx_embedding_key ON mgd.gxd_specimen USING btree (_embedding_key ASC);

CREATE INDEX gxd_specimen_idx_fixation_key ON mgd.gxd_specimen USING btree (_fixation_key ASC);

CREATE INDEX gxd_specimen_idx_genotype_key ON mgd.gxd_specimen USING btree (_genotype_key ASC);

CREATE INDEX img_image_idx_createdby_key ON mgd.img_image USING btree (_createdby_key ASC);

CREATE INDEX img_image_idx_creation_date ON mgd.img_image USING btree (creation_date ASC);

CREATE INDEX img_image_idx_imageclass_key ON mgd.img_image USING btree (_imageclass_key ASC);

CREATE INDEX img_image_idx_imagetype_key ON mgd.img_image USING btree (_imagetype_key ASC);

CREATE INDEX img_image_idx_modification_date ON mgd.img_image USING btree (modification_date ASC);

CREATE INDEX img_image_idx_modifiedby_key ON mgd.img_image USING btree (_modifiedby_key ASC);

CREATE INDEX img_image_idx_refs_key ON mgd.img_image USING btree (_refs_key ASC);

CREATE INDEX img_image_idx_thumbnailimage_key ON mgd.img_image USING btree (_thumbnailimage_key ASC);

CREATE INDEX img_imagepane_assoc_idx_createdby_key ON mgd.img_imagepane_assoc USING btree (_createdby_key ASC);

CREATE INDEX img_imagepane_assoc_idx_imagepane_key ON mgd.img_imagepane_assoc USING btree (_imagepane_key ASC);

CREATE INDEX img_imagepane_assoc_idx_mgitype_key ON mgd.img_imagepane_assoc USING btree (_mgitype_key ASC);

CREATE INDEX img_imagepane_assoc_idx_modifiedby_key ON mgd.img_imagepane_assoc USING btree (_modifiedby_key ASC);

CREATE INDEX img_imagepane_assoc_idx_object_key ON mgd.img_imagepane_assoc USING btree (_object_key ASC);

CREATE INDEX img_imagepane_idx_image_key ON mgd.img_imagepane USING btree (_image_key ASC);

CREATE INDEX map_coord_collection_idx_createdby_key ON mgd.map_coord_collection USING btree (_createdby_key ASC);

CREATE INDEX map_coord_collection_idx_modifiedby_key ON mgd.map_coord_collection USING btree (_modifiedby_key ASC);

CREATE INDEX map_coord_collection_idx_name ON mgd.map_coord_collection USING btree (name ASC);

CREATE INDEX map_coord_feature_idx_createdby_key ON mgd.map_coord_feature USING btree (_createdby_key ASC);

CREATE INDEX map_coord_feature_idx_map_key ON mgd.map_coord_feature USING btree (_map_key ASC);

CREATE INDEX map_coord_feature_idx_mgitype_key ON mgd.map_coord_feature USING btree (_mgitype_key ASC);

CREATE INDEX map_coord_feature_idx_modifiedby_key ON mgd.map_coord_feature USING btree (_modifiedby_key ASC);

CREATE INDEX map_coord_feature_idx_object_key ON mgd.map_coord_feature USING btree (_object_key ASC);

CREATE INDEX map_coordinate_idx_collection_key ON mgd.map_coordinate USING btree (_collection_key ASC);

CREATE INDEX map_coordinate_idx_createdby_key ON mgd.map_coordinate USING btree (_createdby_key ASC);

CREATE INDEX map_coordinate_idx_maptype_key ON mgd.map_coordinate USING btree (_maptype_key ASC);

CREATE INDEX map_coordinate_idx_mgitype_key ON mgd.map_coordinate USING btree (_mgitype_key ASC);

CREATE INDEX map_coordinate_idx_modifiedby_key ON mgd.map_coordinate USING btree (_modifiedby_key ASC);

CREATE INDEX map_coordinate_idx_object_key ON mgd.map_coordinate USING btree (_object_key ASC);

CREATE INDEX map_coordinate_idx_units_key ON mgd.map_coordinate USING btree (_units_key ASC);

CREATE INDEX mgi_keyvalue_idx_createdby_key ON mgd.mgi_keyvalue USING btree (_createdby_key ASC);

CREATE INDEX mgi_keyvalue_idx_key ON mgd.mgi_keyvalue USING btree (key ASC);

CREATE INDEX mgi_keyvalue_idx_mgitype_key ON mgd.mgi_keyvalue USING btree (_mgitype_key ASC);

CREATE INDEX mgi_keyvalue_idx_modifiedby_key ON mgd.mgi_keyvalue USING btree (_modifiedby_key ASC);

CREATE INDEX mgi_keyvalue_idx_objectkeysequencenum ON mgd.mgi_keyvalue USING btree (_object_key ASC, sequencenum);

CREATE INDEX mgi_note_idx_clustered ON mgd.mgi_note USING btree (_object_key ASC, _mgitype_key, _notetype_key);

CREATE INDEX mgi_note_idx_createdby_key ON mgd.mgi_note USING btree (_createdby_key ASC);

CREATE INDEX mgi_note_idx_mgitype_key ON mgd.mgi_note USING btree (_mgitype_key ASC);

CREATE INDEX mgi_note_idx_modifiedby_key ON mgd.mgi_note USING btree (_modifiedby_key ASC);

CREATE INDEX mgi_note_idx_notetype_key ON mgd.mgi_note USING btree (_notetype_key ASC);

CREATE INDEX mgi_notetype_0 ON mgd.mgi_notetype USING btree (lower(notetype) ASC);

CREATE INDEX mgi_notetype_idx_createdby_key ON mgd.mgi_notetype USING btree (_createdby_key ASC);

CREATE INDEX mgi_notetype_idx_mgitype_key ON mgd.mgi_notetype USING btree (_mgitype_key ASC);

CREATE INDEX mgi_notetype_idx_modifiedby_key ON mgd.mgi_notetype USING btree (_modifiedby_key ASC);

CREATE INDEX mgi_organism_idx_commonname ON mgd.mgi_organism USING btree (commonname ASC);

CREATE INDEX mgi_organism_idx_createdby_key ON mgd.mgi_organism USING btree (_createdby_key ASC);

CREATE INDEX mgi_organism_idx_modifiedby_key ON mgd.mgi_organism USING btree (_modifiedby_key ASC);

CREATE INDEX mgi_organism_mgitype_idx_createdby_key ON mgd.mgi_organism_mgitype USING btree (_createdby_key ASC);

CREATE INDEX mgi_organism_mgitype_idx_mgitype_key ON mgd.mgi_organism_mgitype USING btree (_mgitype_key ASC);

CREATE INDEX mgi_organism_mgitype_idx_modifiedby_key ON mgd.mgi_organism_mgitype USING btree (_modifiedby_key ASC);

CREATE INDEX mgi_organism_mgitype_idx_organism_key ON mgd.mgi_organism_mgitype USING btree (_organism_key ASC);

CREATE INDEX mgi_property_idx_createdby_key ON mgd.mgi_property USING btree (_createdby_key ASC);

CREATE INDEX mgi_property_idx_mgitype_key ON mgd.mgi_property USING btree (_mgitype_key ASC);

CREATE INDEX mgi_property_idx_modifiedby_key ON mgd.mgi_property USING btree (_modifiedby_key ASC);

CREATE INDEX mgi_property_idx_object_key ON mgd.mgi_property USING btree (_object_key ASC);

CREATE INDEX mgi_property_idx_propertyterm_key ON mgd.mgi_property USING btree (_propertyterm_key ASC);

CREATE INDEX mgi_property_idx_propertytype_key ON mgd.mgi_property USING btree (_propertytype_key ASC);

CREATE INDEX mgi_property_idx_value ON mgd.mgi_property USING btree (value ASC);

CREATE INDEX mgi_propertytype_idx_createdby_key ON mgd.mgi_propertytype USING btree (_createdby_key ASC);

CREATE INDEX mgi_propertytype_idx_mgitype_key ON mgd.mgi_propertytype USING btree (_mgitype_key ASC);

CREATE INDEX mgi_propertytype_idx_modifiedby_key ON mgd.mgi_propertytype USING btree (_modifiedby_key ASC);

CREATE UNIQUE INDEX mgi_propertytype_idx_propertytype ON mgd.mgi_propertytype USING btree (propertytype ASC);

CREATE INDEX mgi_propertytype_idx_vocab_key ON mgd.mgi_propertytype USING btree (_vocab_key ASC);

CREATE INDEX mgi_refassoctype_idx_createdby_key ON mgd.mgi_refassoctype USING btree (_createdby_key ASC);

CREATE INDEX mgi_refassoctype_idx_mgitype_key ON mgd.mgi_refassoctype USING btree (_mgitype_key ASC);

CREATE INDEX mgi_refassoctype_idx_modifiedby_key ON mgd.mgi_refassoctype USING btree (_modifiedby_key ASC);

CREATE INDEX mgi_reference_assoc_idx_createdby_key ON mgd.mgi_reference_assoc USING btree (_createdby_key ASC);

CREATE INDEX mgi_reference_assoc_idx_mgitype_key ON mgd.mgi_reference_assoc USING btree (_mgitype_key ASC);

CREATE INDEX mgi_reference_assoc_idx_modifiedby_key ON mgd.mgi_reference_assoc USING btree (_modifiedby_key ASC);

CREATE INDEX mgi_reference_assoc_idx_object_key ON mgd.mgi_reference_assoc USING btree (_object_key ASC);

CREATE INDEX mgi_reference_assoc_idx_refassoctype_key ON mgd.mgi_reference_assoc USING btree (_refassoctype_key ASC);

CREATE INDEX mgi_reference_assoc_idx_refs_key ON mgd.mgi_reference_assoc USING btree (_refs_key ASC);

CREATE INDEX mgi_relationship_category_idx_modification_date ON mgd.mgi_relationship_category USING btree (modification_date ASC);

CREATE INDEX mgi_relationship_category_idx_name ON mgd.mgi_relationship_category USING btree (name ASC);

CREATE INDEX mgi_relationship_idx_category_key ON mgd.mgi_relationship USING btree (_category_key ASC);

CREATE INDEX mgi_relationship_idx_clustered ON mgd.mgi_relationship USING btree (_object_key_1 ASC, _object_key_2);

CREATE INDEX mgi_relationship_idx_createdby_key ON mgd.mgi_relationship USING btree (_createdby_key ASC);

CREATE INDEX mgi_relationship_idx_evidence_key ON mgd.mgi_relationship USING btree (_evidence_key ASC);

CREATE INDEX mgi_relationship_idx_modifiedby_key ON mgd.mgi_relationship USING btree (_modifiedby_key ASC);

CREATE INDEX mgi_relationship_idx_object_key_2 ON mgd.mgi_relationship USING btree (_object_key_2 ASC);

CREATE INDEX mgi_relationship_idx_qualifier_key ON mgd.mgi_relationship USING btree (_qualifier_key ASC);

CREATE INDEX mgi_relationship_idx_refs_key ON mgd.mgi_relationship USING btree (_refs_key ASC);

CREATE INDEX mgi_relationship_idx_relationshipterm_key ON mgd.mgi_relationship USING btree (_relationshipterm_key ASC);

CREATE INDEX mgi_relationship_property_idx_clustered ON mgd.mgi_relationship_property USING btree (_relationship_key ASC, sequencenum);

CREATE INDEX mgi_relationship_property_idx_createdby_key ON mgd.mgi_relationship_property USING btree (_createdby_key ASC);

CREATE INDEX mgi_relationship_property_idx_modifiedby_key ON mgd.mgi_relationship_property USING btree (_modifiedby_key ASC);

CREATE INDEX mgi_relationship_property_idx_propertyname_key ON mgd.mgi_relationship_property USING btree (_propertyname_key ASC);

CREATE INDEX mgi_set_idx_clustered ON mgd.mgi_set USING btree (_mgitype_key ASC, name, _set_key, sequencenum);

CREATE INDEX mgi_set_idx_createdby_key ON mgd.mgi_set USING btree (_createdby_key ASC);

CREATE INDEX mgi_set_idx_modifiedby_key ON mgd.mgi_set USING btree (_modifiedby_key ASC);

CREATE INDEX mgi_set_idx_name ON mgd.mgi_set USING btree (name ASC, _mgitype_key, _set_key, sequencenum);

CREATE INDEX mgi_setmember_emapa_idx_createdby_key ON mgd.mgi_setmember_emapa USING btree (_createdby_key ASC);

CREATE INDEX mgi_setmember_emapa_idx_modifiedby_key ON mgd.mgi_setmember_emapa USING btree (_modifiedby_key ASC);

CREATE INDEX mgi_setmember_emapa_idx_setmember_key ON mgd.mgi_setmember_emapa USING btree (_setmember_key ASC);

CREATE INDEX mgi_setmember_emapa_idx_stage_key ON mgd.mgi_setmember_emapa USING btree (_stage_key ASC);

CREATE INDEX mgi_setmember_idx_createdby_key ON mgd.mgi_setmember USING btree (_createdby_key ASC);

CREATE INDEX mgi_setmember_idx_modifiedby_key ON mgd.mgi_setmember USING btree (_modifiedby_key ASC);

CREATE INDEX mgi_setmember_idx_objectset_key ON mgd.mgi_setmember USING btree (_object_key ASC, _set_key, sequencenum);

CREATE UNIQUE INDEX mgi_setmember_idx_setmember_key ON mgd.mgi_setmember USING btree (_setmember_key ASC);

CREATE INDEX mgi_synonym_idx_createdby_key ON mgd.mgi_synonym USING btree (_createdby_key ASC);

CREATE INDEX mgi_synonym_idx_mgitype_key ON mgd.mgi_synonym USING btree (_mgitype_key ASC);

CREATE INDEX mgi_synonym_idx_modifiedby_key ON mgd.mgi_synonym USING btree (_modifiedby_key ASC);

CREATE INDEX mgi_synonym_idx_object_key ON mgd.mgi_synonym USING btree (_object_key ASC);

CREATE INDEX mgi_synonym_idx_refs_key ON mgd.mgi_synonym USING btree (_refs_key ASC);

CREATE INDEX mgi_synonym_idx_synonym ON mgd.mgi_synonym USING btree (synonym ASC);

CREATE INDEX mgi_synonym_idx_synonymtype_key ON mgd.mgi_synonym USING btree (_synonymtype_key ASC);

CREATE INDEX mgi_synonymtype_0 ON mgd.mgi_synonymtype USING btree (lower(synonymtype) ASC);

CREATE INDEX mgi_synonymtype_idx_createdby_key ON mgd.mgi_synonymtype USING btree (_createdby_key ASC);

CREATE INDEX mgi_synonymtype_idx_mgitype_key ON mgd.mgi_synonymtype USING btree (_mgitype_key ASC);

CREATE INDEX mgi_synonymtype_idx_modifiedby_key ON mgd.mgi_synonymtype USING btree (_modifiedby_key ASC);

CREATE INDEX mgi_synonymtype_idx_organism_key ON mgd.mgi_synonymtype USING btree (_organism_key ASC);

CREATE INDEX mgi_translation_idx_badname_key ON mgd.mgi_translation USING btree (badname ASC);

CREATE INDEX mgi_translation_idx_createdby_key ON mgd.mgi_translation USING btree (_createdby_key ASC);

CREATE INDEX mgi_translation_idx_modifiedby_key ON mgd.mgi_translation USING btree (_modifiedby_key ASC);

CREATE INDEX mgi_translation_idx_object_key ON mgd.mgi_translation USING btree (_object_key ASC);

CREATE INDEX mgi_translation_idx_translationtype_key ON mgd.mgi_translation USING btree (_translationtype_key ASC);

CREATE INDEX mgi_translationtype_idx_createdby_key ON mgd.mgi_translationtype USING btree (_createdby_key ASC);

CREATE INDEX mgi_translationtype_idx_mgitype_key ON mgd.mgi_translationtype USING btree (_mgitype_key ASC);

CREATE INDEX mgi_translationtype_idx_modifiedby_key ON mgd.mgi_translationtype USING btree (_modifiedby_key ASC);

CREATE UNIQUE INDEX mgi_translationtype_idx_translationtype ON mgd.mgi_translationtype USING btree (translationtype ASC);

CREATE INDEX mgi_translationtype_idx_vocab_key ON mgd.mgi_translationtype USING btree (_vocab_key ASC);

CREATE INDEX mgi_user_idx_createdby_key ON mgd.mgi_user USING btree (_createdby_key ASC);

CREATE INDEX mgi_user_idx_group_key ON mgd.mgi_user USING btree (_group_key ASC);

CREATE INDEX mgi_user_idx_login ON mgd.mgi_user USING btree (login ASC);

CREATE INDEX mgi_user_idx_modifiedby_key ON mgd.mgi_user USING btree (_modifiedby_key ASC);

CREATE INDEX mgi_user_idx_userstatus_key ON mgd.mgi_user USING btree (_userstatus_key ASC);

CREATE INDEX mgi_user_idx_usertype_key ON mgd.mgi_user USING btree (_usertype_key ASC);

CREATE UNIQUE INDEX mld_assay_types_idx_description ON mgd.mld_assay_types USING btree (description ASC);

CREATE INDEX mld_concordance_idx_marker_key ON mgd.mld_concordance USING btree (_marker_key ASC);

CREATE INDEX mld_contig_idx_expt_key ON mgd.mld_contig USING btree (_expt_key ASC);

CREATE UNIQUE INDEX mld_contig_idx_name ON mgd.mld_contig USING btree (name ASC);

CREATE INDEX mld_contigprobe_idx_probe_key ON mgd.mld_contigprobe USING btree (_probe_key ASC);

CREATE INDEX mld_expt_marker_idx_allele_key ON mgd.mld_expt_marker USING btree (_allele_key ASC);

CREATE INDEX mld_expt_marker_idx_assay_type_key ON mgd.mld_expt_marker USING btree (_assay_type_key ASC);

CREATE INDEX mld_expt_marker_idx_expt_key ON mgd.mld_expt_marker USING btree (_expt_key ASC);

CREATE INDEX mld_expt_marker_idx_marker_key ON mgd.mld_expt_marker USING btree (_marker_key ASC);

CREATE INDEX mld_expts_idx_chromosome ON mgd.mld_expts USING btree (chromosome ASC);

CREATE INDEX mld_expts_idx_creation_date ON mgd.mld_expts USING btree (creation_date ASC);

CREATE INDEX mld_expts_idx_expttype ON mgd.mld_expts USING btree (expttype ASC);

CREATE INDEX mld_expts_idx_modification_date ON mgd.mld_expts USING btree (modification_date ASC);

CREATE INDEX mld_expts_idx_refs_key ON mgd.mld_expts USING btree (_refs_key ASC);

CREATE INDEX mld_fish_idx_strain_key ON mgd.mld_fish USING btree (_strain_key ASC);

CREATE INDEX mld_hit_idx_probe_key ON mgd.mld_hit USING btree (_target_key ASC);

CREATE INDEX mld_hit_idx_target_key ON mgd.mld_hit USING btree (_probe_key ASC);

CREATE INDEX mld_insitu_idx_strain_key ON mgd.mld_insitu USING btree (_strain_key ASC);

CREATE INDEX mld_matrix_idx_cross_key ON mgd.mld_matrix USING btree (_cross_key ASC);

CREATE INDEX mld_mc2point_idx_marker_key ON mgd.mld_mc2point USING btree (_marker_key_1 ASC);

CREATE INDEX mld_mc2point_idx_marker_key_2 ON mgd.mld_mc2point USING btree (_marker_key_2 ASC);

CREATE INDEX mld_ri2point_idx_marker_key_1 ON mgd.mld_ri2point USING btree (_marker_key_1 ASC);

CREATE INDEX mld_ri2point_idx_marker_key_2 ON mgd.mld_ri2point USING btree (_marker_key_2 ASC);

CREATE INDEX mld_ri_idx_riset_key ON mgd.mld_ri USING btree (_riset_key ASC);

CREATE INDEX mld_ridata_idx_marker_key ON mgd.mld_ridata USING btree (_marker_key ASC);

CREATE INDEX mld_statistics_idx_marker_key_1 ON mgd.mld_statistics USING btree (_marker_key_1 ASC);

CREATE INDEX mld_statistics_idx_marker_key_2 ON mgd.mld_statistics USING btree (_marker_key_2 ASC);

CREATE INDEX mrk_biotypemapping_idx_biotypeterm_key ON mgd.mrk_biotypemapping USING btree (_biotypeterm_key ASC);

CREATE INDEX mrk_biotypemapping_idx_biotypevocab_key ON mgd.mrk_biotypemapping USING btree (_biotypevocab_key ASC);

CREATE INDEX mrk_biotypemapping_idx_createdby_key ON mgd.mrk_biotypemapping USING btree (_createdby_key ASC);

CREATE INDEX mrk_biotypemapping_idx_marker_type_key ON mgd.mrk_biotypemapping USING btree (_marker_type_key ASC);

CREATE INDEX mrk_biotypemapping_idx_mcvterm_key ON mgd.mrk_biotypemapping USING btree (_mcvterm_key ASC);

CREATE INDEX mrk_biotypemapping_idx_modifiedby_key ON mgd.mrk_biotypemapping USING btree (_modifiedby_key ASC);

CREATE INDEX mrk_biotypemapping_idx_primarymcvterm_key ON mgd.mrk_biotypemapping USING btree (_primarymcvterm_key ASC);

CREATE INDEX mrk_chromosome_idx_chromosome ON mgd.mrk_chromosome USING btree (chromosome ASC);

CREATE UNIQUE INDEX mrk_chromosome_idx_clustered ON mgd.mrk_chromosome USING btree (_organism_key ASC, chromosome);

CREATE INDEX mrk_cluster_idx_clusterid ON mgd.mrk_cluster USING btree (clusterid ASC);

CREATE INDEX mrk_cluster_idx_clustersource_key ON mgd.mrk_cluster USING btree (_clustersource_key ASC);

CREATE INDEX mrk_cluster_idx_clustertype_key ON mgd.mrk_cluster USING btree (_clustertype_key ASC);

CREATE INDEX mrk_clustermember_idx_cluster_key ON mgd.mrk_clustermember USING btree (_cluster_key ASC);

CREATE INDEX mrk_clustermember_idx_marker_key ON mgd.mrk_clustermember USING btree (_marker_key ASC);

CREATE INDEX mrk_current_idx_marker_key ON mgd.mrk_current USING btree (_marker_key ASC);

CREATE INDEX mrk_do_cache_idx_clustered ON mgd.mrk_do_cache USING btree (_term_key ASC, _marker_key);

CREATE INDEX mrk_do_cache_idx_genotype_key ON mgd.mrk_do_cache USING btree (_genotype_key ASC);

CREATE INDEX mrk_do_cache_idx_marker_key ON mgd.mrk_do_cache USING btree (_marker_key ASC);

CREATE INDEX mrk_do_cache_idx_organism_key ON mgd.mrk_do_cache USING btree (_organism_key ASC);

CREATE INDEX mrk_do_cache_idx_refs_key ON mgd.mrk_do_cache USING btree (_refs_key ASC);

CREATE INDEX mrk_history_idx_createdby_key ON mgd.mrk_history USING btree (_createdby_key ASC);

CREATE INDEX mrk_history_idx_creation_date ON mgd.mrk_history USING btree (creation_date ASC);

CREATE INDEX mrk_history_idx_event_date ON mgd.mrk_history USING btree (event_date ASC);

CREATE INDEX mrk_history_idx_history_key ON mgd.mrk_history USING btree (_history_key ASC);

CREATE INDEX mrk_history_idx_marker_event_key ON mgd.mrk_history USING btree (_marker_event_key ASC);

CREATE INDEX mrk_history_idx_marker_eventreason_key ON mgd.mrk_history USING btree (_marker_eventreason_key ASC);

CREATE INDEX mrk_history_idx_marker_key ON mgd.mrk_history USING btree (_marker_key ASC);

CREATE INDEX mrk_history_idx_modification_date ON mgd.mrk_history USING btree (modification_date ASC);

CREATE INDEX mrk_history_idx_modifiedby_key ON mgd.mrk_history USING btree (_modifiedby_key ASC);

CREATE INDEX mrk_history_idx_refs_key ON mgd.mrk_history USING btree (_refs_key ASC);

CREATE INDEX mrk_label_0 ON mgd.mrk_label USING btree (lower(label) ASC);

CREATE INDEX mrk_label_idx_clustered ON mgd.mrk_label USING btree (_marker_key ASC, priority, _orthologorganism_key, labeltype, label);

CREATE INDEX mrk_label_idx_label ON mgd.mrk_label USING btree (label ASC, _organism_key, _marker_key);

CREATE INDEX mrk_label_idx_label_status_key ON mgd.mrk_label USING btree (_label_status_key ASC);

CREATE INDEX mrk_label_idx_organism_key ON mgd.mrk_label USING btree (_organism_key ASC);

CREATE INDEX mrk_label_idx_priority ON mgd.mrk_label USING btree (priority ASC);

CREATE INDEX mrk_location_cache_0 ON mgd.mrk_location_cache USING btree (lower(chromosome) ASC);

CREATE INDEX mrk_location_cache_idx_chromosome_cmoffset ON mgd.mrk_location_cache USING btree (chromosome ASC, cmoffset);

CREATE INDEX mrk_location_cache_idx_clustered ON mgd.mrk_location_cache USING btree (chromosome ASC, startcoordinate, endcoordinate);

CREATE INDEX mrk_location_cache_idx_marker_type_key ON mgd.mrk_location_cache USING btree (_marker_type_key ASC);

CREATE INDEX mrk_location_cache_idx_organism_key ON mgd.mrk_location_cache USING btree (_organism_key ASC);

CREATE INDEX mrk_marker_idx_clustered ON mgd.mrk_marker USING btree (chromosome ASC);

CREATE INDEX mrk_marker_idx_createdby_key ON mgd.mrk_marker USING btree (_createdby_key ASC);

CREATE INDEX mrk_marker_idx_creation_date ON mgd.mrk_marker USING btree (creation_date ASC);

CREATE UNIQUE INDEX mrk_marker_idx_marker_key ON mgd.mrk_marker USING btree (_marker_key ASC, _organism_key);

CREATE INDEX mrk_marker_idx_marker_status_key ON mgd.mrk_marker USING btree (_marker_status_key ASC);

CREATE INDEX mrk_marker_idx_marker_type_key ON mgd.mrk_marker USING btree (_marker_type_key ASC, _marker_key);

CREATE INDEX mrk_marker_idx_modification_date ON mgd.mrk_marker USING btree (modification_date ASC);

CREATE INDEX mrk_marker_idx_modifiedby_key ON mgd.mrk_marker USING btree (_modifiedby_key ASC);

CREATE INDEX mrk_marker_idx_organism_key ON mgd.mrk_marker USING btree (_organism_key ASC);

CREATE INDEX mrk_marker_idx_symbol ON mgd.mrk_marker USING btree (symbol ASC);

CREATE INDEX mrk_mcv_cache_idx_term ON mgd.mrk_mcv_cache USING btree (term ASC);

CREATE INDEX mrk_notes_idx_note ON mgd.mrk_notes USING btree (note ASC);

CREATE INDEX mrk_reference_idx_refs_key ON mgd.mrk_reference USING btree (_refs_key ASC);

CREATE INDEX mrk_status_idx_status ON mgd.mrk_status USING btree (status ASC);

CREATE INDEX mrk_strainmarker_idx_createdby_key ON mgd.mrk_strainmarker USING btree (_createdby_key ASC);

CREATE INDEX mrk_strainmarker_idx_marker_key ON mgd.mrk_strainmarker USING btree (_marker_key ASC);

CREATE INDEX mrk_strainmarker_idx_modifiedby_key ON mgd.mrk_strainmarker USING btree (_modifiedby_key ASC);

CREATE INDEX mrk_strainmarker_idx_refs_key ON mgd.mrk_strainmarker USING btree (_refs_key ASC);

CREATE INDEX mrk_strainmarker_idx_strain_key ON mgd.mrk_strainmarker USING btree (_strain_key ASC);

CREATE INDEX mrk_types_idx_name ON mgd.mrk_types USING btree (name ASC);

CREATE INDEX prb_alias_idx_alias ON mgd.prb_alias USING btree (alias ASC);

CREATE INDEX prb_alias_idx_clustered ON mgd.prb_alias USING btree (_reference_key ASC);

CREATE INDEX prb_alias_idx_createdby_key ON mgd.prb_alias USING btree (_createdby_key ASC);

CREATE INDEX prb_alias_idx_modifiedby_key ON mgd.prb_alias USING btree (_modifiedby_key ASC);

CREATE INDEX prb_allele_idx_clustered ON mgd.prb_allele USING btree (_rflv_key ASC);

CREATE INDEX prb_allele_idx_createdby_key ON mgd.prb_allele USING btree (_createdby_key ASC);

CREATE INDEX prb_allele_idx_modifiedby_key ON mgd.prb_allele USING btree (_modifiedby_key ASC);

CREATE UNIQUE INDEX prb_allele_strain_idx_clustered ON mgd.prb_allele_strain USING btree (_allele_key ASC, _strain_key);

CREATE INDEX prb_allele_strain_idx_createdby_key ON mgd.prb_allele_strain USING btree (_createdby_key ASC);

CREATE INDEX prb_allele_strain_idx_modifiedby_key ON mgd.prb_allele_strain USING btree (_modifiedby_key ASC);

CREATE INDEX prb_allele_strain_idx_strain_key ON mgd.prb_allele_strain USING btree (_strain_key ASC);

CREATE INDEX prb_marker_idx_createdby_key ON mgd.prb_marker USING btree (_createdby_key ASC);

CREATE INDEX prb_marker_idx_marker_key ON mgd.prb_marker USING btree (_marker_key ASC);

CREATE INDEX prb_marker_idx_modifiedby_key ON mgd.prb_marker USING btree (_modifiedby_key ASC);

CREATE INDEX prb_marker_idx_probe_key ON mgd.prb_marker USING btree (_probe_key ASC);

CREATE INDEX prb_marker_idx_refs_key ON mgd.prb_marker USING btree (_refs_key ASC);

CREATE INDEX prb_marker_idx_relationship ON mgd.prb_marker USING btree (relationship ASC);

CREATE INDEX prb_notes_idx_note ON mgd.prb_notes USING btree (note ASC);

CREATE INDEX prb_notes_idx_probe_key ON mgd.prb_notes USING btree (_probe_key ASC);

CREATE INDEX prb_probe_idx_ampprimer ON mgd.prb_probe USING btree (ampprimer ASC);

CREATE UNIQUE INDEX prb_probe_idx_clustered ON mgd.prb_probe USING btree (_segmenttype_key ASC, _source_key, _probe_key);

CREATE INDEX prb_probe_idx_createdby_key ON mgd.prb_probe USING btree (_createdby_key ASC);

CREATE INDEX prb_probe_idx_creation_date ON mgd.prb_probe USING btree (creation_date ASC);

CREATE INDEX prb_probe_idx_derivedfrom ON mgd.prb_probe USING btree (derivedfrom ASC);

CREATE INDEX prb_probe_idx_modification_date ON mgd.prb_probe USING btree (modification_date ASC);

CREATE INDEX prb_probe_idx_modifiedby_key ON mgd.prb_probe USING btree (_modifiedby_key ASC);

CREATE INDEX prb_probe_idx_name ON mgd.prb_probe USING btree (name ASC);

CREATE INDEX prb_probe_idx_source_key ON mgd.prb_probe USING btree (_source_key ASC);

CREATE INDEX prb_probe_idx_vector_key ON mgd.prb_probe USING btree (_vector_key ASC);

CREATE INDEX prb_ref_notes_idx_note ON mgd.prb_ref_notes USING btree (note ASC);

CREATE INDEX prb_reference_idx_clustered ON mgd.prb_reference USING btree (_probe_key ASC);

CREATE INDEX prb_reference_idx_createdby_key ON mgd.prb_reference USING btree (_createdby_key ASC);

CREATE INDEX prb_reference_idx_modifiedby_key ON mgd.prb_reference USING btree (_modifiedby_key ASC);

CREATE INDEX prb_reference_idx_refs_key ON mgd.prb_reference USING btree (_refs_key ASC);

CREATE INDEX prb_rflv_idx_clustered ON mgd.prb_rflv USING btree (_reference_key ASC);

CREATE INDEX prb_rflv_idx_createdby_key ON mgd.prb_rflv USING btree (_createdby_key ASC);

CREATE INDEX prb_rflv_idx_marker_key ON mgd.prb_rflv USING btree (_marker_key ASC);

CREATE INDEX prb_rflv_idx_modifiedby_key ON mgd.prb_rflv USING btree (_modifiedby_key ASC);

CREATE INDEX prb_source_idx_cellline_key ON mgd.prb_source USING btree (_cellline_key ASC);

CREATE INDEX prb_source_idx_clustered ON mgd.prb_source USING btree (_organism_key ASC, _source_key);

CREATE INDEX prb_source_idx_createdby_key ON mgd.prb_source USING btree (_createdby_key ASC);

CREATE INDEX prb_source_idx_gender_key ON mgd.prb_source USING btree (_gender_key ASC);

CREATE INDEX prb_source_idx_modifiedby_key ON mgd.prb_source USING btree (_modifiedby_key ASC);

CREATE INDEX prb_source_idx_name ON mgd.prb_source USING btree (name ASC);

CREATE INDEX prb_source_idx_refs_key ON mgd.prb_source USING btree (_refs_key ASC);

CREATE INDEX prb_source_idx_segmenttype_key ON mgd.prb_source USING btree (_segmenttype_key ASC);

CREATE INDEX prb_source_idx_strain_key ON mgd.prb_source USING btree (_strain_key ASC);

CREATE INDEX prb_source_idx_tissue_key ON mgd.prb_source USING btree (_tissue_key ASC);

CREATE INDEX prb_source_idx_vector_key ON mgd.prb_source USING btree (_vector_key ASC);

CREATE INDEX prb_strain_genotype_idx_clustered ON mgd.prb_strain_genotype USING btree (_strain_key ASC);

CREATE INDEX prb_strain_genotype_idx_createdby_key ON mgd.prb_strain_genotype USING btree (_createdby_key ASC);

CREATE INDEX prb_strain_genotype_idx_genotype_key ON mgd.prb_strain_genotype USING btree (_genotype_key ASC);

CREATE INDEX prb_strain_genotype_idx_modifiedby_key ON mgd.prb_strain_genotype USING btree (_modifiedby_key ASC);

CREATE INDEX prb_strain_genotype_idx_qualifier_key ON mgd.prb_strain_genotype USING btree (_qualifier_key ASC);

CREATE INDEX prb_strain_idx_createdby_key ON mgd.prb_strain USING btree (_createdby_key ASC);

CREATE INDEX prb_strain_idx_creation_date ON mgd.prb_strain USING btree (creation_date ASC);

CREATE INDEX prb_strain_idx_modification_date ON mgd.prb_strain USING btree (modification_date ASC);

CREATE INDEX prb_strain_idx_modifiedby_key ON mgd.prb_strain USING btree (_modifiedby_key ASC);

CREATE INDEX prb_strain_idx_species_key ON mgd.prb_strain USING btree (_species_key ASC);

CREATE INDEX prb_strain_idx_strain ON mgd.prb_strain USING btree (strain ASC);

CREATE INDEX prb_strain_idx_straintype_key ON mgd.prb_strain USING btree (_straintype_key ASC);

CREATE INDEX prb_strain_marker_idx_allele_key ON mgd.prb_strain_marker USING btree (_allele_key ASC);

CREATE INDEX prb_strain_marker_idx_clustered ON mgd.prb_strain_marker USING btree (_strain_key ASC);

CREATE INDEX prb_strain_marker_idx_createdby_key ON mgd.prb_strain_marker USING btree (_createdby_key ASC);

CREATE INDEX prb_strain_marker_idx_marker_key ON mgd.prb_strain_marker USING btree (_marker_key ASC);

CREATE INDEX prb_strain_marker_idx_modifiedby_key ON mgd.prb_strain_marker USING btree (_modifiedby_key ASC);

CREATE INDEX prb_strain_marker_idx_qualifier_key ON mgd.prb_strain_marker USING btree (_qualifier_key ASC);

CREATE INDEX prb_tissue_idx_tissue ON mgd.prb_tissue USING btree (tissue ASC);

CREATE UNIQUE INDEX ri_riset_idx_designation ON mgd.ri_riset USING btree (designation ASC);

CREATE INDEX ri_riset_idx_strain_key_1 ON mgd.ri_riset USING btree (_strain_key_1 ASC);

CREATE INDEX ri_riset_idx_strain_key_2 ON mgd.ri_riset USING btree (_strain_key_2 ASC);

CREATE INDEX ri_summary_expt_ref_idx_expt_key ON mgd.ri_summary_expt_ref USING btree (_expt_key ASC);

CREATE INDEX ri_summary_expt_ref_idx_refs_key ON mgd.ri_summary_expt_ref USING btree (_refs_key ASC);

CREATE INDEX ri_summary_idx_marker_key ON mgd.ri_summary USING btree (_marker_key ASC);

CREATE INDEX ri_summary_idx_riset_key ON mgd.ri_summary USING btree (_riset_key ASC);

CREATE INDEX seq_allele_assoc_idx_allele_key ON mgd.seq_allele_assoc USING btree (_allele_key ASC);

CREATE INDEX seq_allele_assoc_idx_createdby_key ON mgd.seq_allele_assoc USING btree (_createdby_key ASC);

CREATE INDEX seq_allele_assoc_idx_modifiedby_key ON mgd.seq_allele_assoc USING btree (_modifiedby_key ASC);

CREATE INDEX seq_allele_assoc_idx_qualifier_key ON mgd.seq_allele_assoc USING btree (_qualifier_key ASC);

CREATE INDEX seq_allele_assoc_idx_refs_key ON mgd.seq_allele_assoc USING btree (_refs_key ASC);

CREATE INDEX seq_allele_assoc_idx_sequence_key ON mgd.seq_allele_assoc USING btree (_sequence_key ASC, _allele_key);

CREATE INDEX seq_coord_cache_idx_clustered ON mgd.seq_coord_cache USING btree (chromosome ASC, startcoordinate, endcoordinate);

CREATE INDEX seq_coord_cache_idx_sequence_key ON mgd.seq_coord_cache USING btree (_sequence_key ASC);

CREATE INDEX seq_genemodel_idx_createdby_key ON mgd.seq_genemodel USING btree (_createdby_key ASC);

CREATE INDEX seq_genemodel_idx_gmmarker_type_key ON mgd.seq_genemodel USING btree (_gmmarker_type_key ASC);

CREATE INDEX seq_genemodel_idx_modifiedby_key ON mgd.seq_genemodel USING btree (_modifiedby_key ASC);

CREATE INDEX seq_genetrap_idx_reversecomp_key ON mgd.seq_genetrap USING btree (_reversecomp_key ASC);

CREATE INDEX seq_genetrap_idx_tagmethod_key ON mgd.seq_genetrap USING btree (_tagmethod_key ASC);

CREATE INDEX seq_genetrap_idx_vectorend_key ON mgd.seq_genetrap USING btree (_vectorend_key ASC);

CREATE INDEX seq_marker_cache_idx_accid ON mgd.seq_marker_cache USING btree (accid ASC);

CREATE INDEX seq_marker_cache_idx_annotation_date ON mgd.seq_marker_cache USING btree (annotation_date ASC);

CREATE INDEX seq_marker_cache_idx_biotypeconflict_key ON mgd.seq_marker_cache USING btree (_biotypeconflict_key ASC);

CREATE INDEX seq_marker_cache_idx_clustered ON mgd.seq_marker_cache USING btree (_sequence_key ASC, _marker_key, _refs_key);

CREATE INDEX seq_marker_cache_idx_logicaldb_key ON mgd.seq_marker_cache USING btree (_logicaldb_key ASC);

CREATE INDEX seq_marker_cache_idx_marker_key ON mgd.seq_marker_cache USING btree (_marker_key ASC, _sequence_key);

CREATE INDEX seq_marker_cache_idx_marker_type_key ON mgd.seq_marker_cache USING btree (_marker_type_key ASC);

CREATE INDEX seq_marker_cache_idx_organism_key ON mgd.seq_marker_cache USING btree (_organism_key ASC);

CREATE INDEX seq_marker_cache_idx_qualifier_key ON mgd.seq_marker_cache USING btree (_qualifier_key ASC);

CREATE INDEX seq_marker_cache_idx_refs_key ON mgd.seq_marker_cache USING btree (_refs_key ASC);

CREATE INDEX seq_marker_cache_idx_sequenceprovider_key ON mgd.seq_marker_cache USING btree (_sequenceprovider_key ASC);

CREATE INDEX seq_marker_cache_idx_sequencetype_key ON mgd.seq_marker_cache USING btree (_sequencetype_key ASC);

CREATE INDEX seq_probe_cache_idx_annotation_date ON mgd.seq_probe_cache USING btree (annotation_date ASC);

CREATE INDEX seq_probe_cache_idx_probe_key ON mgd.seq_probe_cache USING btree (_probe_key ASC, _sequence_key);

CREATE INDEX seq_probe_cache_idx_refs_key ON mgd.seq_probe_cache USING btree (_refs_key ASC);

CREATE INDEX seq_sequence_assoc_idx_clustered ON mgd.seq_sequence_assoc USING btree (_sequence_key_1 ASC, _sequence_key_2, _qualifier_key);

CREATE INDEX seq_sequence_assoc_idx_createdby_key ON mgd.seq_sequence_assoc USING btree (_createdby_key ASC);

CREATE INDEX seq_sequence_assoc_idx_modifiedby_key ON mgd.seq_sequence_assoc USING btree (_modifiedby_key ASC);

CREATE INDEX seq_sequence_assoc_idx_qualifier_key ON mgd.seq_sequence_assoc USING btree (_qualifier_key ASC);

CREATE INDEX seq_sequence_assoc_idx_sequence_key_2 ON mgd.seq_sequence_assoc USING btree (_sequence_key_2 ASC, _sequence_key_1, _qualifier_key);

CREATE INDEX seq_sequence_idx_clustered ON mgd.seq_sequence USING btree (_sequencetype_key ASC, _sequenceprovider_key, length);

CREATE INDEX seq_sequence_idx_createdby_key ON mgd.seq_sequence USING btree (_createdby_key ASC);

CREATE INDEX seq_sequence_idx_length ON mgd.seq_sequence USING btree (length ASC);

CREATE INDEX seq_sequence_idx_modifiedby_key ON mgd.seq_sequence USING btree (_modifiedby_key ASC);

CREATE INDEX seq_sequence_idx_seqrecord_date ON mgd.seq_sequence USING btree (seqrecord_date ASC);

CREATE INDEX seq_sequence_idx_sequenceprovider_key ON mgd.seq_sequence USING btree (_sequenceprovider_key ASC);

CREATE INDEX seq_sequence_idx_sequencequality_key ON mgd.seq_sequence USING btree (_sequencequality_key ASC);

CREATE INDEX seq_sequence_idx_sequencestatus_key ON mgd.seq_sequence USING btree (_sequencestatus_key ASC);

CREATE INDEX seq_sequence_raw_idx_createdby_key ON mgd.seq_sequence_raw USING btree (_createdby_key ASC);

CREATE INDEX seq_sequence_raw_idx_modifiedby_key ON mgd.seq_sequence_raw USING btree (_modifiedby_key ASC);

CREATE UNIQUE INDEX seq_source_assoc_idx_clustered ON mgd.seq_source_assoc USING btree (_sequence_key ASC, _source_key);

CREATE INDEX seq_source_assoc_idx_createdby_key ON mgd.seq_source_assoc USING btree (_createdby_key ASC);

CREATE INDEX seq_source_assoc_idx_modifiedby_key ON mgd.seq_source_assoc USING btree (_modifiedby_key ASC);

CREATE INDEX seq_source_assoc_idx_source_key ON mgd.seq_source_assoc USING btree (_source_key ASC);

CREATE INDEX voc_allele_cache_idx_allele_key ON mgd.voc_allele_cache USING btree (_allele_key ASC, _term_key);

CREATE INDEX voc_allele_cache_idx_annottype ON mgd.voc_allele_cache USING btree (annottype ASC, _allele_key);

CREATE INDEX voc_annot_count_cache_idx_annottype ON mgd.voc_annot_count_cache USING btree (annottype ASC, _term_key);

CREATE INDEX voc_annot_idx_annottype_key ON mgd.voc_annot USING btree (_annottype_key ASC);

CREATE INDEX voc_annot_idx_clustered ON mgd.voc_annot USING btree (_object_key ASC, _term_key, _annottype_key, _qualifier_key);

CREATE INDEX voc_annot_idx_qualifier_key ON mgd.voc_annot USING btree (_qualifier_key ASC);

CREATE INDEX voc_annot_idx_term_etc_key ON mgd.voc_annot USING btree (_term_key ASC, _annottype_key, _qualifier_key, _object_key);

CREATE INDEX voc_annotheader_idx_approvedby_key ON mgd.voc_annotheader USING btree (_approvedby_key ASC);

CREATE INDEX voc_annotheader_idx_clustered ON mgd.voc_annotheader USING btree (_annottype_key ASC, _object_key, _term_key);

CREATE INDEX voc_annotheader_idx_createdby_key ON mgd.voc_annotheader USING btree (_createdby_key ASC);

CREATE INDEX voc_annotheader_idx_modifiedby_key ON mgd.voc_annotheader USING btree (_modifiedby_key ASC);

CREATE INDEX voc_annotheader_idx_object_key ON mgd.voc_annotheader USING btree (_object_key ASC);

CREATE INDEX voc_annotheader_idx_term_key ON mgd.voc_annotheader USING btree (_term_key ASC);

CREATE INDEX voc_annottype_0 ON mgd.voc_annottype USING btree (lower(name) ASC);

CREATE INDEX voc_annottype_idx_evidencevocab_key ON mgd.voc_annottype USING btree (_evidencevocab_key ASC);

CREATE INDEX voc_annottype_idx_mgitypevocabevidence ON mgd.voc_annottype USING btree (_mgitype_key ASC, _vocab_key, _evidencevocab_key);

CREATE INDEX voc_annottype_idx_name ON mgd.voc_annottype USING btree (name ASC);

CREATE INDEX voc_annottype_idx_qualifiervocab_key ON mgd.voc_annottype USING btree (_qualifiervocab_key ASC);

CREATE INDEX voc_annottype_idx_vocab_key ON mgd.voc_annottype USING btree (_vocab_key ASC);

CREATE INDEX voc_evidence_idx_clustered ON mgd.voc_evidence USING btree (_annot_key ASC);

CREATE INDEX voc_evidence_idx_createdby_key ON mgd.voc_evidence USING btree (_createdby_key ASC);

CREATE INDEX voc_evidence_idx_creation_date ON mgd.voc_evidence USING btree (creation_date ASC);

CREATE INDEX voc_evidence_idx_evidenceterm_key ON mgd.voc_evidence USING btree (_evidenceterm_key ASC);

CREATE INDEX voc_evidence_idx_modification_date ON mgd.voc_evidence USING btree (modification_date ASC);

CREATE INDEX voc_evidence_idx_modifiedby_key ON mgd.voc_evidence USING btree (_modifiedby_key ASC);

CREATE INDEX voc_evidence_idx_refs_key ON mgd.voc_evidence USING btree (_refs_key ASC);

CREATE INDEX voc_evidence_property_idx_clustered ON mgd.voc_evidence_property USING btree (_annotevidence_key ASC);

CREATE INDEX voc_evidence_property_idx_createdby_key ON mgd.voc_evidence_property USING btree (_createdby_key ASC);

CREATE INDEX voc_evidence_property_idx_modifiedby_key ON mgd.voc_evidence_property USING btree (_modifiedby_key ASC);

CREATE INDEX voc_evidence_property_idx_propertyterm_key ON mgd.voc_evidence_property USING btree (_propertyterm_key ASC);

CREATE INDEX voc_marker_cache_idx_annottype_etc ON mgd.voc_marker_cache USING btree (annottype ASC, _marker_key);

CREATE INDEX voc_marker_cache_idx_marker_key ON mgd.voc_marker_cache USING btree (_marker_key ASC, _term_key);

CREATE INDEX voc_marker_cache_idx_term_key ON mgd.voc_marker_cache USING btree (_term_key ASC, _marker_key);

CREATE INDEX voc_term_0 ON mgd.voc_term USING btree (lower(term) ASC);

CREATE INDEX voc_term_emapa_idx_createdby_key ON mgd.voc_term_emapa USING btree (_createdby_key ASC);

CREATE INDEX voc_term_emapa_idx_endstage ON mgd.voc_term_emapa USING btree (endstage ASC);

CREATE INDEX voc_term_emapa_idx_modifiedby_key ON mgd.voc_term_emapa USING btree (_modifiedby_key ASC);

CREATE INDEX voc_term_emapa_idx_parent ON mgd.voc_term_emapa USING btree (_defaultparent_key ASC);

CREATE INDEX voc_term_emapa_idx_startstage ON mgd.voc_term_emapa USING btree (startstage ASC);

CREATE INDEX voc_term_emaps_idx_createdby_key ON mgd.voc_term_emaps USING btree (_createdby_key ASC);

CREATE INDEX voc_term_emaps_idx_emapa ON mgd.voc_term_emaps USING btree (_emapa_term_key ASC);

CREATE INDEX voc_term_emaps_idx_modifiedby_key ON mgd.voc_term_emaps USING btree (_modifiedby_key ASC);

CREATE INDEX voc_term_emaps_idx_parent ON mgd.voc_term_emaps USING btree (_defaultparent_key ASC);

CREATE INDEX voc_term_emaps_idx_stage_key ON mgd.voc_term_emaps USING btree (_stage_key ASC);

CREATE INDEX voc_term_idx_clustered ON mgd.voc_term USING btree (_vocab_key ASC, sequencenum, term, _term_key);

CREATE INDEX voc_term_idx_createdby_key ON mgd.voc_term USING btree (_createdby_key ASC);

CREATE INDEX voc_term_idx_creation_date ON mgd.voc_term USING btree (creation_date ASC);

CREATE INDEX voc_term_idx_modification_date ON mgd.voc_term USING btree (modification_date ASC);

CREATE INDEX voc_term_idx_modifiedby_key ON mgd.voc_term USING btree (_modifiedby_key ASC);

CREATE INDEX voc_term_idx_term ON mgd.voc_term USING btree (term ASC, _term_key, _vocab_key);

CREATE UNIQUE INDEX voc_vocab_idx_name ON mgd.voc_vocab USING btree (name ASC, _vocab_key);

CREATE INDEX voc_vocab_idx_refs_key ON mgd.voc_vocab USING btree (_refs_key ASC);

CREATE INDEX voc_vocabdag_idx_dag_key ON mgd.voc_vocabdag USING btree (_dag_key ASC);

CREATE INDEX wks_rosetta_idx_clustered ON mgd.wks_rosetta USING btree (_marker_key ASC);
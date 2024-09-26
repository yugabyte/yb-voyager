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


CREATE VIEW mgd.acc_view AS
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    m.name AS mgitype,
    l.name AS logicaldb,
    l.description,
    l._organism_key,
    d.name AS actualdb,
    d.url,
    d.allowsmultiple,
    d.delimiter
   FROM mgd.acc_accession a,
    mgd.acc_mgitype m,
    mgd.acc_logicaldb l,
    mgd.acc_actualdb d
  WHERE ((a._mgitype_key = m._mgitype_key) AND (a._logicaldb_key = l._logicaldb_key) AND (l._logicaldb_key = d._logicaldb_key) AND (d.active = 1));


CREATE VIEW mgd.all_acc_view AS
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    l.name AS logicaldb
   FROM mgd.acc_accession a,
    mgd.acc_logicaldb l
  WHERE ((a._mgitype_key = 11) AND (a._logicaldb_key = l._logicaldb_key));


CREATE VIEW mgd.all_allele_cellline_view AS
 SELECT a._assoc_key,
    a._allele_key,
    a._mutantcellline_key,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    c.cellline,
    c.ismutant,
    p._strain_key AS celllinestrain_key,
    s.strain AS celllinestrain,
    d._creator_key,
    vd1.term AS creator,
    d._vector_key,
    vd2.term AS vector,
    p._cellline_key AS parentcellline_key,
    p.cellline AS parentcellline,
    c._derivation_key AS derivationkey,
    d.name AS derivationname,
    p._cellline_type_key AS parentcelllinetype_key,
    aa.symbol,
    u1.login AS createdby,
    u2.login AS modifiedby
   FROM mgd.all_allele_cellline a,
    mgd.all_cellline c,
    mgd.all_cellline_derivation d,
    mgd.all_cellline p,
    mgd.all_allele aa,
    mgd.prb_strain s,
    mgd.voc_term vd1,
    mgd.voc_term vd2,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((a._mutantcellline_key = c._cellline_key) AND (a._allele_key = aa._allele_key) AND (c._derivation_key = d._derivation_key) AND (d._creator_key = vd1._term_key) AND (d._vector_key = vd2._term_key) AND (d._parentcellline_key = p._cellline_key) AND (p._strain_key = s._strain_key) AND (a._createdby_key = u1._user_key) AND (a._modifiedby_key = u2._user_key));


CREATE VIEW mgd.all_allele_driver_view AS
 SELECT r._relationship_key,
    r._object_key_1 AS _allele_key,
    r._object_key_2 AS _marker_key,
    m._organism_key,
    o.commonname,
    m.symbol
   FROM mgd.mgi_relationship r,
    mgd.mrk_marker m,
    mgd.mgi_organism o
  WHERE ((r._category_key = 1006) AND (r._object_key_2 = m._marker_key) AND (m._organism_key = o._organism_key));


CREATE VIEW mgd.all_allele_mutation_view AS
 SELECT am._assoc_key,
    am._allele_key,
    am._mutation_key,
    am.creation_date,
    am.modification_date,
    t.term AS mutation
   FROM mgd.all_allele_mutation am,
    mgd.voc_term t
  WHERE (am._mutation_key = t._term_key);


CREATE VIEW mgd.all_allele_subtype_view AS
 SELECT a._allele_key,
    va._annot_key,
    va._annottype_key,
    va._object_key,
    va._term_key,
    va._qualifier_key,
    va.creation_date,
    va.modification_date,
    t.term
   FROM mgd.all_allele a,
    mgd.voc_annot va,
    mgd.voc_term t
  WHERE ((a._allele_key = va._object_key) AND (va._annottype_key = 1014) AND (va._term_key = t._term_key));


CREATE VIEW mgd.all_allele_view AS
 SELECT a._allele_key,
    a._marker_key,
    a._strain_key,
    a._mode_key,
    a._allele_type_key,
    a._allele_status_key,
    a._transmission_key,
    a._collection_key,
    a.symbol,
    a.name,
    a.iswildtype,
    a.isextinct,
    a.ismixed,
    a._refs_key,
    a._markerallele_status_key,
    a._createdby_key,
    a._modifiedby_key,
    a._approvedby_key,
    a.approval_date,
    a.creation_date,
    a.modification_date,
    m.symbol AS markersymbol,
    t.term,
    t.sequencenum AS statusnum,
    s.strain,
    t2.term AS collection,
    u1.login AS createdby,
    u2.login AS modifiedby,
    u3.login AS approvedby,
    t3.term AS markerallele_status,
    r.numericpart AS jnum,
    r.jnumid,
    r.citation,
    r.short_citation
   FROM (((mgd.all_allele a
     LEFT JOIN mgd.mrk_marker m ON ((a._marker_key = m._marker_key)))
     LEFT JOIN mgd.bib_citation_cache r ON ((a._refs_key = r._refs_key)))
     LEFT JOIN mgd.mgi_user u3 ON ((a._approvedby_key = u3._user_key))),
    mgd.prb_strain s,
    mgd.voc_term t,
    mgd.voc_term t2,
    mgd.voc_term t3,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((a._allele_status_key = t._term_key) AND (a._collection_key = t2._term_key) AND (a._markerallele_status_key = t3._term_key) AND (a._strain_key = s._strain_key) AND (a._createdby_key = u1._user_key) AND (a._modifiedby_key = u2._user_key));


CREATE VIEW mgd.all_genotype_view AS
 SELECT DISTINCT gxd_allelepair._genotype_key,
    gxd_allelepair._allele_key_1 AS _allele_key
   FROM mgd.gxd_allelepair
UNION
 SELECT DISTINCT gxd_allelepair._genotype_key,
    gxd_allelepair._allele_key_2 AS _allele_key
   FROM mgd.gxd_allelepair
  WHERE (gxd_allelepair._allele_key_2 IS NOT NULL);


CREATE VIEW mgd.all_annot_view AS
 SELECT agv._allele_key,
    va._annot_key,
    va._annottype_key,
    va._qualifier_key
   FROM ((mgd.all_genotype_view agv
     JOIN mgd.voc_annot va ON ((va._object_key = agv._genotype_key)))
     JOIN mgd.voc_annottype vat ON ((vat._annottype_key = va._annottype_key)))
  WHERE (vat._mgitype_key = 12)
UNION ALL
 SELECT a._allele_key,
    va._annot_key,
    va._annottype_key,
    va._qualifier_key
   FROM ((mgd.all_allele a
     JOIN mgd.voc_annot va ON ((va._object_key = a._allele_key)))
     JOIN mgd.voc_annottype vat ON ((vat._annottype_key = va._annottype_key)))
  WHERE (vat._mgitype_key = 11);


CREATE VIEW mgd.all_cellline_acc_view AS
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    l.name AS logicaldb
   FROM mgd.acc_accession a,
    mgd.acc_logicaldb l
  WHERE ((a._mgitype_key = 28) AND (a._logicaldb_key = l._logicaldb_key));


CREATE VIEW mgd.all_cellline_derivation_view AS
 SELECT c._derivation_key,
    c.name,
    c.description,
    c._vector_key,
    c._vectortype_key,
    c._parentcellline_key,
    c._derivationtype_key,
    c._creator_key,
    c._refs_key,
    c._createdby_key,
    c._modifiedby_key,
    c.creation_date,
    c.modification_date,
    p._cellline_key AS parentcellline_key,
    p.cellline AS parentcellline,
    p._strain_key AS parentcelllinestrain_key,
    s.strain AS parentcelllinestrain,
    p._cellline_type_key AS parentcelllinetype_key,
    vt4.term AS parentcelllinetype,
    vt1.term AS creator,
    vt2.term AS vector,
    vt3.term AS vectortype,
    cc.jnumid,
    cc.numericpart AS jnum,
    cc.short_citation,
    u1.login AS createdby,
    u2.login AS modifiedby
   FROM (((((((((mgd.all_cellline_derivation c
     JOIN mgd.all_cellline p ON ((c._parentcellline_key = p._cellline_key)))
     JOIN mgd.prb_strain s ON ((p._strain_key = s._strain_key)))
     JOIN mgd.voc_term vt1 ON ((c._creator_key = vt1._term_key)))
     JOIN mgd.voc_term vt2 ON ((c._vector_key = vt2._term_key)))
     JOIN mgd.voc_term vt3 ON ((c._vectortype_key = vt3._term_key)))
     JOIN mgd.voc_term vt4 ON ((p._cellline_type_key = vt4._term_key)))
     JOIN mgd.mgi_user u1 ON ((c._createdby_key = u1._user_key)))
     JOIN mgd.mgi_user u2 ON ((c._modifiedby_key = u2._user_key)))
     LEFT JOIN mgd.bib_citation_cache cc ON ((c._refs_key = cc._refs_key)));


CREATE VIEW mgd.all_cellline_view AS
 SELECT c._cellline_key,
    c.cellline,
    c._cellline_type_key,
    c._strain_key,
    c._derivation_key,
    c.ismutant,
    c._createdby_key,
    c._modifiedby_key,
    c.creation_date,
    c.modification_date,
    vt.term AS celllinetype,
    s1.strain AS celllinestrain,
    d._creator_key,
    vt1.term AS creator,
    p._cellline_key AS parentcellline_key,
    p.cellline AS parentcellline,
    d.name AS derivationname,
    d._derivationtype_key,
    d._vector_key,
    vt2.term AS vector,
    d._vectortype_key,
    vt3.term AS vectortype,
    p._strain_key AS parentcelllinestrain_key,
    s2.strain AS parentcelllinestrain,
    u1.login AS createdby,
    u2.login AS modifiedby
   FROM ((((((((((mgd.all_cellline c
     JOIN mgd.voc_term vt ON ((c._cellline_type_key = vt._term_key)))
     JOIN mgd.prb_strain s1 ON ((c._strain_key = s1._strain_key)))
     JOIN mgd.mgi_user u1 ON ((c._createdby_key = u1._user_key)))
     JOIN mgd.mgi_user u2 ON ((c._modifiedby_key = u2._user_key)))
     LEFT JOIN mgd.all_cellline_derivation d ON ((c._derivation_key = d._derivation_key)))
     LEFT JOIN mgd.voc_term vt1 ON ((d._creator_key = vt1._term_key)))
     LEFT JOIN mgd.voc_term vt2 ON ((d._vector_key = vt2._term_key)))
     LEFT JOIN mgd.voc_term vt3 ON ((d._vectortype_key = vt3._term_key)))
     LEFT JOIN mgd.all_cellline p ON ((d._parentcellline_key = p._cellline_key)))
     LEFT JOIN mgd.prb_strain s2 ON ((p._strain_key = s2._strain_key)));


CREATE VIEW mgd.all_summary_view AS
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    a2.accid AS mgiid,
    t.term AS subtype,
    ((al.symbol || ', '::text) || al.name) AS description,
    al.symbol AS short_description
   FROM mgd.acc_accession a,
    mgd.acc_accession a2,
    mgd.all_allele al,
    mgd.voc_term t
  WHERE ((a._mgitype_key = 11) AND (a.private = 0) AND (a._object_key = a2._object_key) AND (a2._logicaldb_key = 1) AND (a2._mgitype_key = 11) AND (a2.prefixpart = 'MGI:'::text) AND (a2.preferred = 1) AND (a._object_key = al._allele_key) AND (al._allele_type_key = t._term_key));


CREATE VIEW mgd.all_summarybymarker_view AS
 SELECT DISTINCT aa.accid,
    a._allele_key,
    a.symbol,
    t1.term AS transmission,
    t2.term AS allelestatus,
    array_to_string(array_agg(DISTINCT vt.term), ','::text) AS diseaseannots,
    NULL::text AS mpannots
   FROM mgd.all_allele a,
    mgd.acc_accession aa,
    mgd.voc_term t1,
    mgd.voc_term t2,
    mgd.gxd_allelegenotype g,
    mgd.voc_annot va,
    mgd.voc_term vt
  WHERE ((a._marker_key = aa._object_key) AND (aa._mgitype_key = 2) AND (aa._logicaldb_key = 1) AND (aa.preferred = 1) AND (a._transmission_key = t1._term_key) AND (a._allele_status_key = t2._term_key) AND (a._allele_key = g._allele_key) AND (g._genotype_key = va._object_key) AND (va._annottype_key = 1020) AND (va._term_key = vt._term_key) AND (NOT (EXISTS ( SELECT 1
           FROM mgd.gxd_allelegenotype g_1,
            mgd.voc_annot va_1
          WHERE ((a._allele_key = g_1._allele_key) AND (g_1._genotype_key = va_1._object_key) AND (va_1._annottype_key = 1002))))))
  GROUP BY aa.accid, a._allele_key, a.symbol, t1.term, t2.term
UNION
 SELECT DISTINCT aa.accid,
    a._allele_key,
    a.symbol,
    t1.term AS transmission,
    t2.term AS allelestatus,
    NULL::text AS diseaseannots,
    NULL::text AS mpannots
   FROM mgd.all_allele a,
    mgd.acc_accession aa,
    mgd.voc_term t1,
    mgd.voc_term t2
  WHERE ((a._marker_key = aa._object_key) AND (aa._mgitype_key = 2) AND (aa._logicaldb_key = 1) AND (aa.preferred = 1) AND (a._transmission_key = t1._term_key) AND (a._allele_status_key = t2._term_key) AND (NOT (EXISTS ( SELECT 1
           FROM mgd.gxd_allelegenotype g,
            mgd.voc_annot va
          WHERE ((a._allele_key = g._allele_key) AND (g._genotype_key = va._object_key) AND (va._annottype_key = 1020))))) AND (NOT (EXISTS ( SELECT 1
           FROM mgd.gxd_allelegenotype g,
            mgd.voc_annot va
          WHERE ((a._allele_key = g._allele_key) AND (g._genotype_key = va._object_key) AND (va._annottype_key = 1002))))))
  GROUP BY aa.accid, a._allele_key, a.symbol, t1.term, t2.term
UNION
 SELECT DISTINCT aa.accid,
    a._allele_key,
    a.symbol,
    t1.term AS transmission,
    t2.term AS allelestatus,
    array_to_string(array_agg(DISTINCT vt.term), ','::text) AS diseaseannots,
    'has data'::text AS mpannots
   FROM mgd.all_allele a,
    mgd.acc_accession aa,
    mgd.voc_term t1,
    mgd.voc_term t2,
    mgd.gxd_allelegenotype g,
    mgd.voc_annot va,
    mgd.voc_term vt
  WHERE ((a._marker_key = aa._object_key) AND (aa._mgitype_key = 2) AND (aa._logicaldb_key = 1) AND (aa.preferred = 1) AND (a._transmission_key = t1._term_key) AND (a._allele_status_key = t2._term_key) AND (a._allele_key = g._allele_key) AND (g._genotype_key = va._object_key) AND (va._annottype_key = 1020) AND (va._term_key = vt._term_key) AND (EXISTS ( SELECT 1
           FROM mgd.gxd_allelegenotype g_1,
            mgd.voc_annot va_1
          WHERE ((a._allele_key = g_1._allele_key) AND (g_1._genotype_key = va_1._object_key) AND (va_1._annottype_key = 1002) AND (va_1._qualifier_key = 2181423)))))
  GROUP BY aa.accid, a._allele_key, a.symbol, t1.term, t2.term
UNION
 SELECT DISTINCT aa.accid,
    a._allele_key,
    a.symbol,
    t1.term AS transmission,
    t2.term AS allelestatus,
    NULL::text AS diseaseannots,
    'has data'::text AS mpannots
   FROM mgd.all_allele a,
    mgd.acc_accession aa,
    mgd.voc_term t1,
    mgd.voc_term t2
  WHERE ((a._marker_key = aa._object_key) AND (aa._mgitype_key = 2) AND (aa._logicaldb_key = 1) AND (aa.preferred = 1) AND (a._transmission_key = t1._term_key) AND (a._allele_status_key = t2._term_key) AND (NOT (EXISTS ( SELECT 1
           FROM mgd.gxd_allelegenotype g,
            mgd.voc_annot va
          WHERE ((a._allele_key = g._allele_key) AND (g._genotype_key = va._object_key) AND (va._annottype_key = 1020))))) AND (EXISTS ( SELECT 1
           FROM mgd.gxd_allelegenotype g,
            mgd.voc_annot va
          WHERE ((a._allele_key = g._allele_key) AND (g._genotype_key = va._object_key) AND (va._annottype_key = 1002) AND (va._qualifier_key = 2181423)))))
  GROUP BY aa.accid, a._allele_key, a.symbol, t1.term, t2.term
UNION
 SELECT DISTINCT aa.accid,
    a._allele_key,
    a.symbol,
    t1.term AS transmission,
    t2.term AS allelestatus,
    array_to_string(array_agg(DISTINCT vt.term), ','::text) AS diseaseannots,
    'no abnormal phenotype observed'::text AS mpannots
   FROM mgd.all_allele a,
    mgd.acc_accession aa,
    mgd.voc_term t1,
    mgd.voc_term t2,
    mgd.gxd_allelegenotype g,
    mgd.voc_annot va,
    mgd.voc_term vt
  WHERE ((a._marker_key = aa._object_key) AND (aa._mgitype_key = 2) AND (aa._logicaldb_key = 1) AND (aa.preferred = 1) AND (a._transmission_key = t1._term_key) AND (a._allele_status_key = t2._term_key) AND (a._allele_key = g._allele_key) AND (g._genotype_key = va._object_key) AND (va._annottype_key = 1020) AND (va._term_key = vt._term_key) AND (NOT (EXISTS ( SELECT 1
           FROM mgd.gxd_allelegenotype g_1,
            mgd.voc_annot va_1
          WHERE ((a._allele_key = g_1._allele_key) AND (g_1._genotype_key = va_1._object_key) AND (va_1._annottype_key = 1002) AND (va_1._qualifier_key = 2181423))))) AND (EXISTS ( SELECT 1
           FROM mgd.gxd_allelegenotype g_1,
            mgd.voc_annot va_1
          WHERE ((a._allele_key = g_1._allele_key) AND (g_1._genotype_key = va_1._object_key) AND (va_1._annottype_key = 1002) AND (va_1._qualifier_key = 2181424)))))
  GROUP BY aa.accid, a._allele_key, a.symbol, t1.term, t2.term
UNION
 SELECT DISTINCT aa.accid,
    a._allele_key,
    a.symbol,
    t1.term AS transmission,
    t2.term AS allelestatus,
    NULL::text AS diseaseannots,
    'no abnormal phenotype observed'::text AS mpannots
   FROM mgd.all_allele a,
    mgd.acc_accession aa,
    mgd.voc_term t1,
    mgd.voc_term t2
  WHERE ((a._marker_key = aa._object_key) AND (aa._mgitype_key = 2) AND (aa._logicaldb_key = 1) AND (aa.preferred = 1) AND (a._transmission_key = t1._term_key) AND (a._allele_status_key = t2._term_key) AND (NOT (EXISTS ( SELECT 1
           FROM mgd.gxd_allelegenotype g,
            mgd.voc_annot va
          WHERE ((a._allele_key = g._allele_key) AND (g._genotype_key = va._object_key) AND (va._annottype_key = 1020))))) AND (NOT (EXISTS ( SELECT 1
           FROM mgd.gxd_allelegenotype g,
            mgd.voc_annot va
          WHERE ((a._allele_key = g._allele_key) AND (g._genotype_key = va._object_key) AND (va._annottype_key = 1002) AND (va._qualifier_key = 2181423))))) AND (EXISTS ( SELECT 1
           FROM mgd.gxd_allelegenotype g,
            mgd.voc_annot va
          WHERE ((a._allele_key = g._allele_key) AND (g._genotype_key = va._object_key) AND (va._annottype_key = 1002) AND (va._qualifier_key = 2181424)))))
  GROUP BY aa.accid, a._allele_key, a.symbol, t1.term, t2.term
  ORDER BY 4 DESC, 5, 3;


CREATE VIEW mgd.all_summarybyreference_view AS
 SELECT DISTINCT c.jnumid,
    aa.accid,
    a._allele_key,
    a.symbol,
    t1.term AS transmission,
    t2.term AS allelestatus,
    t3.term AS alleletype,
    array_to_string(array_agg(DISTINCT vt.term), ','::text) AS diseaseannots,
    NULL::text AS mpannots
   FROM mgd.all_allele a,
    mgd.acc_accession aa,
    mgd.voc_term t1,
    mgd.voc_term t2,
    mgd.voc_term t3,
    mgd.mgi_reference_assoc r,
    mgd.bib_citation_cache c,
    mgd.gxd_allelegenotype g,
    mgd.voc_annot va,
    mgd.voc_term vt
  WHERE ((a._marker_key = aa._object_key) AND (aa._mgitype_key = 2) AND (aa._logicaldb_key = 1) AND (aa.preferred = 1) AND (a._allele_key = r._object_key) AND (r._mgitype_key = 11) AND (r._refs_key = c._refs_key) AND (a._transmission_key = t1._term_key) AND (a._allele_status_key = t2._term_key) AND (a._allele_type_key = t3._term_key) AND (a._allele_key = g._allele_key) AND (g._genotype_key = va._object_key) AND (va._annottype_key = 1020) AND (va._term_key = vt._term_key) AND (NOT (EXISTS ( SELECT 1
           FROM mgd.gxd_allelegenotype g_1,
            mgd.voc_annot va_1
          WHERE ((a._allele_key = g_1._allele_key) AND (g_1._genotype_key = va_1._object_key) AND (va_1._annottype_key = 1002))))))
  GROUP BY aa.accid, a._allele_key, a.symbol, t1.term, t2.term, t3.term, c.jnumid
UNION
 SELECT DISTINCT c.jnumid,
    aa.accid,
    a._allele_key,
    a.symbol,
    t1.term AS transmission,
    t2.term AS allelestatus,
    t3.term AS alleletype,
    NULL::text AS diseaseannots,
    NULL::text AS mpannots
   FROM mgd.all_allele a,
    mgd.acc_accession aa,
    mgd.voc_term t1,
    mgd.voc_term t2,
    mgd.voc_term t3,
    mgd.mgi_reference_assoc r,
    mgd.bib_citation_cache c
  WHERE ((a._marker_key = aa._object_key) AND (aa._mgitype_key = 2) AND (aa._logicaldb_key = 1) AND (aa.preferred = 1) AND (a._allele_key = r._object_key) AND (r._mgitype_key = 11) AND (r._refs_key = c._refs_key) AND (a._transmission_key = t1._term_key) AND (a._allele_status_key = t2._term_key) AND (a._allele_type_key = t3._term_key) AND (NOT (EXISTS ( SELECT 1
           FROM mgd.gxd_allelegenotype g,
            mgd.voc_annot va
          WHERE ((a._allele_key = g._allele_key) AND (g._genotype_key = va._object_key) AND (va._annottype_key = 1020))))) AND (NOT (EXISTS ( SELECT 1
           FROM mgd.gxd_allelegenotype g,
            mgd.voc_annot va
          WHERE ((a._allele_key = g._allele_key) AND (g._genotype_key = va._object_key) AND (va._annottype_key = 1002))))))
  GROUP BY aa.accid, a._allele_key, a.symbol, t1.term, t2.term, t3.term, c.jnumid
UNION
 SELECT DISTINCT c.jnumid,
    aa.accid,
    a._allele_key,
    a.symbol,
    t1.term AS transmission,
    t2.term AS allelestatus,
    t3.term AS alleletype,
    array_to_string(array_agg(DISTINCT vt.term), ','::text) AS diseaseannots,
    'has data'::text AS mpannots
   FROM mgd.all_allele a,
    mgd.acc_accession aa,
    mgd.voc_term t1,
    mgd.voc_term t2,
    mgd.voc_term t3,
    mgd.gxd_allelegenotype g,
    mgd.voc_annot va,
    mgd.voc_term vt,
    mgd.mgi_reference_assoc r,
    mgd.bib_citation_cache c
  WHERE ((a._marker_key = aa._object_key) AND (aa._mgitype_key = 2) AND (aa._logicaldb_key = 1) AND (aa.preferred = 1) AND (a._allele_key = r._object_key) AND (r._mgitype_key = 11) AND (r._refs_key = c._refs_key) AND (a._transmission_key = t1._term_key) AND (a._allele_status_key = t2._term_key) AND (a._allele_type_key = t3._term_key) AND (a._allele_key = g._allele_key) AND (g._genotype_key = va._object_key) AND (va._annottype_key = 1020) AND (va._term_key = vt._term_key) AND (EXISTS ( SELECT 1
           FROM mgd.gxd_allelegenotype g_1,
            mgd.voc_annot va_1
          WHERE ((a._allele_key = g_1._allele_key) AND (g_1._genotype_key = va_1._object_key) AND (va_1._annottype_key = 1002) AND (va_1._qualifier_key = 2181423)))))
  GROUP BY aa.accid, a._allele_key, a.symbol, t1.term, t2.term, t3.term, c.jnumid
UNION
 SELECT DISTINCT c.jnumid,
    aa.accid,
    a._allele_key,
    a.symbol,
    t1.term AS transmission,
    t2.term AS allelestatus,
    t3.term AS alleletype,
    NULL::text AS diseaseannots,
    'has data'::text AS mpannots
   FROM mgd.all_allele a,
    mgd.acc_accession aa,
    mgd.voc_term t1,
    mgd.voc_term t2,
    mgd.voc_term t3,
    mgd.mgi_reference_assoc r,
    mgd.bib_citation_cache c
  WHERE ((a._marker_key = aa._object_key) AND (aa._mgitype_key = 2) AND (aa._logicaldb_key = 1) AND (aa.preferred = 1) AND (a._allele_key = r._object_key) AND (r._mgitype_key = 11) AND (r._refs_key = c._refs_key) AND (a._transmission_key = t1._term_key) AND (a._allele_status_key = t2._term_key) AND (a._allele_type_key = t3._term_key) AND (NOT (EXISTS ( SELECT 1
           FROM mgd.gxd_allelegenotype g,
            mgd.voc_annot va
          WHERE ((a._allele_key = g._allele_key) AND (g._genotype_key = va._object_key) AND (va._annottype_key = 1020))))) AND (EXISTS ( SELECT 1
           FROM mgd.gxd_allelegenotype g,
            mgd.voc_annot va
          WHERE ((a._allele_key = g._allele_key) AND (g._genotype_key = va._object_key) AND (va._annottype_key = 1002) AND (va._qualifier_key = 2181423)))))
  GROUP BY aa.accid, a._allele_key, a.symbol, t1.term, t2.term, t3.term, c.jnumid
UNION
 SELECT DISTINCT c.jnumid,
    aa.accid,
    a._allele_key,
    a.symbol,
    t1.term AS transmission,
    t2.term AS allelestatus,
    t3.term AS alleletype,
    array_to_string(array_agg(DISTINCT vt.term), ','::text) AS diseaseannots,
    'no abnormal phenotype observed'::text AS mpannots
   FROM mgd.all_allele a,
    mgd.acc_accession aa,
    mgd.voc_term t1,
    mgd.voc_term t2,
    mgd.voc_term t3,
    mgd.mgi_reference_assoc r,
    mgd.bib_citation_cache c,
    mgd.gxd_allelegenotype g,
    mgd.voc_annot va,
    mgd.voc_term vt
  WHERE ((a._marker_key = aa._object_key) AND (aa._mgitype_key = 2) AND (aa._logicaldb_key = 1) AND (aa.preferred = 1) AND (a._allele_key = r._object_key) AND (r._mgitype_key = 11) AND (r._refs_key = c._refs_key) AND (a._transmission_key = t1._term_key) AND (a._allele_status_key = t2._term_key) AND (a._allele_type_key = t3._term_key) AND (a._allele_key = g._allele_key) AND (g._genotype_key = va._object_key) AND (va._annottype_key = 1020) AND (va._term_key = vt._term_key) AND (NOT (EXISTS ( SELECT 1
           FROM mgd.gxd_allelegenotype g_1,
            mgd.voc_annot va_1
          WHERE ((a._allele_key = g_1._allele_key) AND (g_1._genotype_key = va_1._object_key) AND (va_1._annottype_key = 1002) AND (va_1._qualifier_key = 2181423))))) AND (EXISTS ( SELECT 1
           FROM mgd.gxd_allelegenotype g_1,
            mgd.voc_annot va_1
          WHERE ((a._allele_key = g_1._allele_key) AND (g_1._genotype_key = va_1._object_key) AND (va_1._annottype_key = 1002) AND (va_1._qualifier_key = 2181424)))))
  GROUP BY aa.accid, a._allele_key, a.symbol, t1.term, t2.term, t3.term, c.jnumid
UNION
 SELECT DISTINCT c.jnumid,
    aa.accid,
    a._allele_key,
    a.symbol,
    t1.term AS transmission,
    t2.term AS allelestatus,
    t3.term AS alleletype,
    NULL::text AS diseaseannots,
    'no abnormal phenotype observed'::text AS mpannots
   FROM mgd.all_allele a,
    mgd.acc_accession aa,
    mgd.voc_term t1,
    mgd.voc_term t2,
    mgd.voc_term t3,
    mgd.mgi_reference_assoc r,
    mgd.bib_citation_cache c
  WHERE ((a._marker_key = aa._object_key) AND (aa._mgitype_key = 2) AND (aa._logicaldb_key = 1) AND (a._allele_key = r._object_key) AND (r._mgitype_key = 11) AND (r._refs_key = c._refs_key) AND (aa.preferred = 1) AND (a._transmission_key = t1._term_key) AND (a._allele_status_key = t2._term_key) AND (a._allele_type_key = t3._term_key) AND (NOT (EXISTS ( SELECT 1
           FROM mgd.gxd_allelegenotype g,
            mgd.voc_annot va
          WHERE ((a._allele_key = g._allele_key) AND (g._genotype_key = va._object_key) AND (va._annottype_key = 1020))))) AND (NOT (EXISTS ( SELECT 1
           FROM mgd.gxd_allelegenotype g,
            mgd.voc_annot va
          WHERE ((a._allele_key = g._allele_key) AND (g._genotype_key = va._object_key) AND (va._annottype_key = 1002) AND (va._qualifier_key = 2181423))))) AND (EXISTS ( SELECT 1
           FROM mgd.gxd_allelegenotype g,
            mgd.voc_annot va
          WHERE ((a._allele_key = g._allele_key) AND (g._genotype_key = va._object_key) AND (va._annottype_key = 1002) AND (va._qualifier_key = 2181424)))))
  GROUP BY aa.accid, a._allele_key, a.symbol, t1.term, t2.term, t3.term, c.jnumid
  ORDER BY 5 DESC, 6, 4;


CREATE VIEW mgd.bib_acc_view AS
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    l.name AS logicaldb
   FROM mgd.acc_accession a,
    mgd.acc_logicaldb l
  WHERE ((a._mgitype_key = 1) AND (a._logicaldb_key = l._logicaldb_key));


CREATE VIEW mgd.bib_all_view AS
 SELECT r._refs_key,
    r._referencetype_key,
    r.authors,
    r._primary,
    r.title,
    r.journal,
    r.vol,
    r.issue,
    r.date,
    r.year,
    r.pgs,
    r.abstract,
    r.isreviewarticle,
    r._createdby_key,
    r._modifiedby_key,
    r.creation_date,
    r.modification_date,
    c.referencetype,
    c.jnumid,
    c.numericpart AS jnum,
    c.citation,
    c.short_citation
   FROM mgd.bib_refs r,
    mgd.bib_citation_cache c
  WHERE (r._refs_key = c._refs_key);


CREATE VIEW mgd.mgi_reference_strain_view AS
 SELECT r._assoc_key,
    r._refs_key,
    r._object_key,
    r._mgitype_key,
    r._refassoctype_key,
    r._createdby_key,
    r._modifiedby_key,
    r.creation_date,
    r.modification_date,
    t.assoctype,
    t.allowonlyone,
    c.jnumid,
    c.numericpart AS jnum,
    c.short_citation,
    c.isreviewarticle,
    c.isreviewarticlestring,
    u.login AS modifiedby,
    s.strain,
    aa.accid
   FROM mgd.mgi_reference_assoc r,
    mgd.mgi_refassoctype t,
    mgd.bib_citation_cache c,
    mgd.mgi_user u,
    mgd.prb_strain s,
    mgd.acc_accession aa
  WHERE ((r._mgitype_key = 10) AND (r._refassoctype_key = t._refassoctype_key) AND (r._refs_key = c._refs_key) AND (r._modifiedby_key = u._user_key) AND (r._object_key = s._strain_key) AND (s._strain_key = aa._object_key) AND (aa._mgitype_key = 10) AND (aa._logicaldb_key = 1) AND (aa.preferred = 1))
  ORDER BY s.strain, t.assoctype;


CREATE VIEW mgd.mgi_synonym_strain_view AS
 SELECT s._synonym_key,
    s._object_key,
    s._mgitype_key,
    s._synonymtype_key,
    s._refs_key,
    s.synonym,
    s._createdby_key,
    s._modifiedby_key,
    s.creation_date,
    s.modification_date,
    t._organism_key,
    t.synonymtype,
    t.allowonlyone,
    c.jnumid,
    c.numericpart AS jnum,
    c.short_citation,
    u1.login AS modifiedby
   FROM (((mgd.mgi_synonym s
     JOIN mgd.mgi_synonymtype t ON (((s._synonymtype_key = t._synonymtype_key) AND (t._mgitype_key = 10))))
     JOIN mgd.mgi_user u1 ON ((s._modifiedby_key = u1._user_key)))
     LEFT JOIN mgd.bib_citation_cache c ON ((s._refs_key = c._refs_key)));


CREATE VIEW mgd.bib_associateddata_view AS
 SELECT r._refs_key,
        CASE
            WHEN (EXISTS ( SELECT 1
               FROM mgd.mgi_reference_assoc mr
              WHERE ((r._refs_key = mr._refs_key) AND (mr._mgitype_key = 11)))) THEN 1
            ELSE 0
        END AS has_alleles,
        CASE
            WHEN (EXISTS ( SELECT 1
               FROM mgd.mgi_reference_assoc mr
              WHERE ((r._refs_key = mr._refs_key) AND (mr._mgitype_key = 6)))) THEN 1
            ELSE 0
        END AS has_antibodies,
        CASE
            WHEN (EXISTS ( SELECT 1
               FROM mgd.voc_annot a,
                mgd.voc_evidence e
              WHERE ((r._refs_key = e._refs_key) AND (e._annot_key = a._annot_key) AND (a._annottype_key = 1000)))) THEN 1
            ELSE 0
        END AS has_go,
        CASE
            WHEN (EXISTS ( SELECT 1
               FROM mgd.gxd_index gi
              WHERE (r._refs_key = gi._refs_key))) THEN 1
            ELSE 0
        END AS has_gxdindex,
        CASE
            WHEN (EXISTS ( SELECT 1
               FROM mgd.img_image ii
              WHERE ((r._refs_key = ii._refs_key) AND (ii._imageclass_key = 6481781)))) THEN 1
            ELSE 0
        END AS has_gxdimages,
        CASE
            WHEN (EXISTS ( SELECT 1
               FROM mgd.gxd_expression e
              WHERE ((r._refs_key = e._refs_key) AND (e._specimen_key IS NOT NULL)))) THEN 1
            ELSE 0
        END AS has_gxdspecimens,
        CASE
            WHEN (EXISTS ( SELECT 1
               FROM mgd.gxd_expression e
              WHERE (r._refs_key = e._refs_key))) THEN 1
            ELSE 0
        END AS has_gxdresults,
        CASE
            WHEN ((EXISTS ( SELECT 1
               FROM mgd.mld_expts e
              WHERE (r._refs_key = e._refs_key))) OR (EXISTS ( SELECT 1
               FROM mgd.mld_notes n
              WHERE (r._refs_key = n._refs_key)))) THEN 1
            ELSE 0
        END AS has_mapping,
        CASE
            WHEN (EXISTS ( SELECT 1
               FROM mgd.mrk_reference m
              WHERE (r._refs_key = m._refs_key))) THEN 1
            ELSE 0
        END AS has_markers,
        CASE
            WHEN (EXISTS ( SELECT 1
               FROM mgd.prb_reference p
              WHERE (r._refs_key = p._refs_key))) THEN 1
            ELSE 0
        END AS has_probes,
        CASE
            WHEN ((EXISTS ( SELECT 1
               FROM mgd.mgi_synonym_strain_view s
              WHERE (r._refs_key = s._refs_key))) OR (EXISTS ( SELECT 1
               FROM mgd.mgi_reference_strain_view s
              WHERE (r._refs_key = s._refs_key))) OR (EXISTS ( SELECT 1
               FROM mgd.all_allele s
              WHERE (r._refs_key = s._refs_key))) OR (EXISTS ( SELECT 1
               FROM mgd.mrk_strainmarker s
              WHERE (r._refs_key = s._refs_key))) OR (EXISTS ( SELECT 1
               FROM mgd.prb_source s
              WHERE (r._refs_key = s._refs_key)))) THEN 1
            ELSE 0
        END AS has_strain,
        CASE
            WHEN ((EXISTS ( SELECT 1
               FROM mgd.gxd_expression g
              WHERE (g._refs_key = r._refs_key))) OR (EXISTS ( SELECT 1
               FROM mgd.voc_evidence e,
                mgd.voc_annot a
              WHERE ((e._refs_key = r._refs_key) AND (e._annot_key = a._annot_key) AND (a._annottype_key = 1002)))) OR (EXISTS ( SELECT 1
               FROM mgd.mrk_do_cache g
              WHERE (g._refs_key = r._refs_key)))) THEN 1
            ELSE 0
        END AS has_genotype
   FROM mgd.bib_refs r
  GROUP BY r._refs_key;


CREATE VIEW mgd.bib_goxref_view AS
 SELECT r._refs_key,
    r.jnum,
    r.short_citation,
    m._marker_key,
    r.jnumid,
    r.title,
    r.creation_date
   FROM mgd.mrk_reference m,
    mgd.bib_all_view r
  WHERE ((m._refs_key = r._refs_key) AND (EXISTS ( SELECT 1
           FROM mgd.bib_workflow_status ws,
            mgd.voc_term wst1,
            mgd.voc_term wst2
          WHERE ((r._refs_key = ws._refs_key) AND (ws._group_key = wst1._term_key) AND (wst1.abbreviation = 'GO'::text) AND (ws._status_key = wst2._term_key) AND (wst2.term = ANY (ARRAY['Chosen'::text, 'Routed'::text, 'Indexed'::text])) AND (ws.iscurrent = 1)))));


CREATE VIEW mgd.bib_status_view AS
 SELECT DISTINCT r._refs_key,
    ap_t.term AS ap_status,
    go_t.term AS go_status,
    gxd_t.term AS gxd_status,
    qtl_t.term AS qtl_status,
    tumor_t.term AS tumor_status,
    pro_t.term AS pro_status
   FROM ((((((((((((mgd.bib_refs r
     LEFT JOIN mgd.bib_workflow_status ap ON (((r._refs_key = ap._refs_key) AND (ap._group_key = 31576664) AND (ap.iscurrent = 1))))
     LEFT JOIN mgd.voc_term ap_t ON ((ap._status_key = ap_t._term_key)))
     LEFT JOIN mgd.bib_workflow_status go ON (((r._refs_key = go._refs_key) AND (go._group_key = 31576666) AND (go.iscurrent = 1))))
     LEFT JOIN mgd.voc_term go_t ON ((go._status_key = go_t._term_key)))
     LEFT JOIN mgd.bib_workflow_status gxd ON (((r._refs_key = gxd._refs_key) AND (gxd._group_key = 31576665) AND (gxd.iscurrent = 1))))
     LEFT JOIN mgd.voc_term gxd_t ON ((gxd._status_key = gxd_t._term_key)))
     LEFT JOIN mgd.bib_workflow_status qtl ON (((r._refs_key = qtl._refs_key) AND (qtl._group_key = 31576668) AND (qtl.iscurrent = 1))))
     LEFT JOIN mgd.voc_term qtl_t ON ((qtl._status_key = qtl_t._term_key)))
     LEFT JOIN mgd.bib_workflow_status tumor ON (((r._refs_key = tumor._refs_key) AND (tumor._group_key = 31576667) AND (tumor.iscurrent = 1))))
     LEFT JOIN mgd.voc_term tumor_t ON ((tumor._status_key = tumor_t._term_key)))
     LEFT JOIN mgd.bib_workflow_status pro ON (((r._refs_key = pro._refs_key) AND (pro._group_key = 78678148) AND (pro.iscurrent = 1))))
     LEFT JOIN mgd.voc_term pro_t ON ((pro._status_key = pro_t._term_key)));


CREATE VIEW mgd.bib_summary_view AS
 SELECT c._refs_key,
    c.numericpart,
    c.jnumid,
    c.mgiid,
    c.pubmedid,
    c.doiid,
    c.journal,
    c.citation,
    c.short_citation,
    c.referencetype,
    c._relevance_key,
    c.relevanceterm,
    c.isreviewarticle,
    c.isreviewarticlestring,
    r.authors,
    r.title,
    r.year,
    r.vol,
    r.abstract,
        CASE
            WHEN (EXISTS ( SELECT 1
               FROM mgd.mgi_reference_assoc mr
              WHERE ((r._refs_key = mr._refs_key) AND (mr._mgitype_key = 11)))) THEN 1
            ELSE 0
        END AS has_alleles,
        CASE
            WHEN (EXISTS ( SELECT 1
               FROM mgd.mgi_reference_assoc mr
              WHERE ((r._refs_key = mr._refs_key) AND (mr._mgitype_key = 6)))) THEN 1
            ELSE 0
        END AS has_antibodies,
        CASE
            WHEN (EXISTS ( SELECT 1
               FROM mgd.gxd_assay e
              WHERE (r._refs_key = e._refs_key))) THEN 1
            ELSE 0
        END AS has_gxdassays,
        CASE
            WHEN (EXISTS ( SELECT 1
               FROM mgd.gxd_assay e
              WHERE (r._refs_key = e._refs_key))) THEN 1
            ELSE 0
        END AS has_gxdresults,
        CASE
            WHEN (EXISTS ( SELECT 1
               FROM mgd.gxd_assay e,
                mgd.gxd_specimen s
              WHERE ((r._refs_key = e._refs_key) AND (e._assay_key = s._assay_key)))) THEN 1
            ELSE 0
        END AS has_gxdspecimens,
        CASE
            WHEN (EXISTS ( SELECT 1
               FROM mgd.gxd_index gi
              WHERE (r._refs_key = gi._refs_key))) THEN 1
            ELSE 0
        END AS has_gxdindex,
        CASE
            WHEN (EXISTS ( SELECT 1
               FROM mgd.img_image ii
              WHERE ((r._refs_key = ii._refs_key) AND (ii._imageclass_key = 6481781)))) THEN 1
            ELSE 0
        END AS has_gxdimages,
        CASE
            WHEN (EXISTS ( SELECT 1
               FROM mgd.mrk_reference m
              WHERE (r._refs_key = m._refs_key))) THEN 1
            ELSE 0
        END AS has_markers,
        CASE
            WHEN (EXISTS ( SELECT 1
               FROM mgd.prb_reference p
              WHERE (r._refs_key = p._refs_key))) THEN 1
            ELSE 0
        END AS has_probes,
        CASE
            WHEN (EXISTS ( SELECT 1
               FROM mgd.mld_expts e
              WHERE ((r._refs_key = e._refs_key) AND (e.expttype = ANY (ARRAY['TEXT-QTL'::text, 'TEXT-Physical Mapping'::text, 'TEXT-Congenic'::text, 'TEXT-QTL-Candidate Genes'::text, 'TEXT-Meta Analysis'::text, 'TEXT'::text, 'TEXT-Genetic Cross'::text]))))) THEN 1
            ELSE 0
        END AS has_mapping,
        CASE
            WHEN ((EXISTS ( SELECT 1
               FROM mgd.gxd_assay g
              WHERE (g._refs_key = r._refs_key))) OR (EXISTS ( SELECT 1
               FROM mgd.voc_evidence e,
                mgd.voc_annot a
              WHERE ((e._refs_key = r._refs_key) AND (e._annot_key = a._annot_key) AND (a._annottype_key = 1002)))) OR (EXISTS ( SELECT 1
               FROM mgd.mrk_do_cache g
              WHERE (g._refs_key = r._refs_key)))) THEN 1
            ELSE 0
        END AS has_genotype
   FROM mgd.bib_citation_cache c,
    mgd.bib_refs r
  WHERE (c._refs_key = r._refs_key);


CREATE VIEW mgd.bib_view AS
 SELECT r._refs_key,
    r._referencetype_key,
    r.authors,
    r._primary,
    r.title,
    r.journal,
    r.vol,
    r.issue,
    r.date,
    r.year,
    r.pgs,
    r.abstract,
    r.isreviewarticle,
    r._createdby_key,
    r._modifiedby_key,
    r.creation_date,
    r.modification_date,
    c.referencetype,
    c.jnumid,
    c.numericpart AS jnum,
    c.citation,
    c.short_citation,
    u1.login AS createdby,
    u2.login AS modifiedby
   FROM mgd.bib_refs r,
    mgd.bib_citation_cache c,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((r._refs_key = c._refs_key) AND (r._createdby_key = u1._user_key) AND (r._modifiedby_key = u2._user_key));


CREATE VIEW mgd.dag_node_view AS
 SELECT n._node_key,
    n._dag_key,
    n._object_key,
    n._label_key,
    n.creation_date,
    n.modification_date,
    d.name AS dag,
    d.abbreviation AS dagabbrev,
    vd._vocab_key
   FROM mgd.dag_node n,
    mgd.dag_dag d,
    mgd.voc_vocabdag vd
  WHERE ((n._dag_key = d._dag_key) AND (d._dag_key = vd._dag_key));


CREATE VIEW mgd.gxd_allelepair_view AS
 SELECT a._allelepair_key,
    a._genotype_key,
    a._allele_key_1,
    a._allele_key_2,
    a._marker_key,
    a._mutantcellline_key_1,
    a._mutantcellline_key_2,
    a._pairstate_key,
    a._compound_key,
    a.sequencenum,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    m.symbol,
    m.chromosome,
    a1.symbol AS allele1,
    a2.symbol AS allele2,
    t1.term AS allelestate,
    t2.term AS compound
   FROM (((((mgd.gxd_allelepair a
     JOIN mgd.mrk_marker m ON ((a._marker_key = m._marker_key)))
     JOIN mgd.all_allele a1 ON ((a._allele_key_1 = a1._allele_key)))
     JOIN mgd.all_allele a2 ON ((a._allele_key_2 = a2._allele_key)))
     JOIN mgd.voc_term t1 ON ((a._pairstate_key = t1._term_key)))
     JOIN mgd.voc_term t2 ON ((a._compound_key = t2._term_key)))
UNION
 SELECT a._allelepair_key,
    a._genotype_key,
    a._allele_key_1,
    a._allele_key_2,
    a._marker_key,
    a._mutantcellline_key_1,
    a._mutantcellline_key_2,
    a._pairstate_key,
    a._compound_key,
    a.sequencenum,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    m.symbol,
    m.chromosome,
    a1.symbol AS allele1,
    NULL::text AS allele2,
    t1.term AS allelestate,
    t2.term AS compound
   FROM ((((mgd.gxd_allelepair a
     JOIN mgd.mrk_marker m ON ((a._marker_key = m._marker_key)))
     JOIN mgd.all_allele a1 ON ((a._allele_key_1 = a1._allele_key)))
     JOIN mgd.voc_term t1 ON ((a._pairstate_key = t1._term_key)))
     JOIN mgd.voc_term t2 ON ((a._compound_key = t2._term_key)))
  WHERE (a._allele_key_2 IS NULL);


CREATE VIEW mgd.gxd_antibody_acc_view AS
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    l.name AS logicaldb,
    g.antibodyname AS description
   FROM mgd.acc_accession a,
    mgd.acc_logicaldb l,
    mgd.gxd_antibody g
  WHERE ((a._mgitype_key = 6) AND (a._logicaldb_key = l._logicaldb_key) AND (a._object_key = g._antibody_key));


CREATE VIEW mgd.gxd_antibody_view AS
 SELECT ab._antibody_key,
    ab._antibodyclass_key,
    ab._antibodytype_key,
    ab._organism_key,
    ab._antigen_key,
    ab.antibodyname,
    ab.antibodynote,
    ab._createdby_key,
    ab._modifiedby_key,
    ab.creation_date,
    ab.modification_date,
    a.accid AS mgiid,
    a.prefixpart,
    a.numericpart,
    ac.term AS class,
    ap.term AS antibodytype,
    ae.commonname AS antibodyspecies,
    ag.antigenname,
    ag.regioncovered,
    ag.antigennote,
    u1.login AS createdby,
    u2.login AS modifiedby
   FROM mgd.gxd_antibody ab,
    mgd.acc_accession a,
    mgd.voc_term ac,
    mgd.voc_term ap,
    mgd.mgi_organism ae,
    mgd.gxd_antigen ag,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((ab._antibody_key = a._object_key) AND (a._mgitype_key = 6) AND (a._logicaldb_key = 1) AND (a.prefixpart = 'MGI:'::text) AND (a.preferred = 1) AND (ab._antibodyclass_key = ac._term_key) AND (ab._antibodytype_key = ap._term_key) AND (ab._organism_key = ae._organism_key) AND (ab._antigen_key = ag._antigen_key) AND (ab._createdby_key = u1._user_key) AND (ab._modifiedby_key = u2._user_key));


CREATE VIEW mgd.gxd_antibodyalias_view AS
 SELECT a.antibodyname,
    aa._antibodyalias_key,
    aa._antibody_key,
    aa._refs_key,
    aa.alias,
    aa.creation_date,
    aa.modification_date
   FROM mgd.gxd_antibody a,
    mgd.gxd_antibodyalias aa
  WHERE (a._antibody_key = aa._antibody_key);


CREATE VIEW mgd.gxd_antibodyaliasref_view AS
 SELECT a.antibodyname,
    aa._antibodyalias_key,
    aa._antibody_key,
    aa._refs_key,
    aa.alias,
    aa.creation_date,
    aa.modification_date,
    c.jnumid,
    c.numericpart AS jnum,
    c.short_citation
   FROM mgd.gxd_antibody a,
    mgd.gxd_antibodyalias aa,
    mgd.bib_citation_cache c
  WHERE ((a._antibody_key = aa._antibody_key) AND (aa._refs_key = c._refs_key));


CREATE VIEW mgd.gxd_antigen_view AS
 SELECT g._antigen_key,
    g._source_key,
    g.antigenname,
    g.regioncovered,
    g.antigennote,
    g._createdby_key,
    g._modifiedby_key,
    g.creation_date,
    g.modification_date,
    a.accid AS mgiid,
    a.prefixpart,
    a.numericpart,
    s._organism_key,
    s.age,
    s._gender_key,
    s._cellline_key,
    s.name AS library,
    s.description,
    s._refs_key,
    s._strain_key,
    s._tissue_key,
    u1.login AS createdby,
    u2.login AS modifiedby
   FROM mgd.gxd_antigen g,
    mgd.acc_accession a,
    mgd.prb_source s,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((g._antigen_key = a._object_key) AND (a._mgitype_key = 7) AND (a._logicaldb_key = 1) AND (a.prefixpart = 'MGI:'::text) AND (a.preferred = 1) AND (g._source_key = s._source_key) AND (g._createdby_key = u1._user_key) AND (g._modifiedby_key = u2._user_key));


CREATE VIEW mgd.gxd_antibodyantigen_view AS
 SELECT ab._antibody_key,
    ab.antibodyname,
    ag._antigen_key,
    ag._source_key,
    ag.antigenname,
    ag.regioncovered,
    ag.antigennote,
    ag._createdby_key,
    ag._modifiedby_key,
    ag.creation_date,
    ag.modification_date,
    ag.mgiid,
    ag.prefixpart,
    ag.numericpart,
    ag._organism_key,
    ag.age,
    ag._gender_key,
    ag._cellline_key,
    ag.library,
    ag.description,
    ag._refs_key,
    ag._strain_key,
    ag._tissue_key,
    ag.createdby,
    ag.modifiedby
   FROM mgd.gxd_antibody ab,
    mgd.gxd_antigen_view ag
  WHERE (ab._antigen_key = ag._antigen_key);


CREATE VIEW mgd.gxd_antibodymarker_view AS
 SELECT a._antibody_key,
    a.antibodyname,
    am._marker_key,
    m.symbol,
    m.chromosome
   FROM mgd.gxd_antibody a,
    mgd.gxd_antibodymarker am,
    mgd.mrk_marker m
  WHERE ((a._antibody_key = am._antibody_key) AND (am._marker_key = m._marker_key));


CREATE VIEW mgd.gxd_antibodyprep_view AS
 SELECT a._assay_key,
    ap._antibodyprep_key,
    ap._antibody_key,
    ap._secondary_key,
    ap._label_key,
    ap.creation_date,
    ap.modification_date,
    s.term AS secondary,
    l.term AS label,
    ab.antibodyname,
    ac.accid
   FROM mgd.gxd_assay a,
    mgd.gxd_antibodyprep ap,
    mgd.voc_term s,
    mgd.voc_term l,
    mgd.gxd_antibody ab,
    mgd.acc_accession ac
  WHERE ((a._antibodyprep_key = ap._antibodyprep_key) AND (ap._secondary_key = s._term_key) AND (ap._label_key = l._term_key) AND (ap._antibody_key = ab._antibody_key) AND (ab._antibody_key = ac._object_key) AND (ac._mgitype_key = 6) AND (ac._logicaldb_key = 1) AND (ac.prefixpart = 'MGI:'::text) AND (ac.preferred = 1));


CREATE VIEW mgd.gxd_antigen_acc_view AS
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    l.name AS logicaldb,
    g.antigenname AS description
   FROM mgd.acc_accession a,
    mgd.acc_logicaldb l,
    mgd.gxd_antigen g
  WHERE ((a._mgitype_key = 7) AND (a._logicaldb_key = l._logicaldb_key) AND (a._object_key = g._antigen_key));


CREATE VIEW mgd.gxd_antigen_summary_view AS
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    a2.accid AS mgiid,
    a2.accid AS subtype,
    g.antigenname AS description,
    g.antigenname AS short_description
   FROM mgd.acc_accession a,
    mgd.acc_accession a2,
    mgd.gxd_antigen g
  WHERE ((a._mgitype_key = 7) AND (a.private = 0) AND (a._object_key = a2._object_key) AND (a2._logicaldb_key = 1) AND (a2._mgitype_key = 7) AND (a2.prefixpart = 'MGI:'::text) AND (a2.preferred = 1) AND (a._object_key = g._antigen_key));


CREATE VIEW mgd.gxd_assay_acc_view AS
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    l.name AS logicaldb
   FROM mgd.acc_accession a,
    mgd.acc_logicaldb l
  WHERE ((a._mgitype_key = 8) AND (a._logicaldb_key = l._logicaldb_key));


CREATE VIEW mgd.gxd_assay_allele_view AS
 SELECT DISTINCT a._assay_key,
    g._allele_key
   FROM ((mgd.gxd_assay a
     JOIN mgd.gxd_specimen s ON ((s._assay_key = a._assay_key)))
     JOIN mgd.gxd_allelegenotype g ON ((g._genotype_key = s._genotype_key)))
UNION ALL
 SELECT DISTINCT a._assay_key,
    g._allele_key
   FROM ((mgd.gxd_assay a
     JOIN mgd.gxd_gellane gl ON ((gl._assay_key = a._assay_key)))
     JOIN mgd.gxd_allelegenotype g ON ((g._genotype_key = gl._genotype_key)));


CREATE VIEW mgd.gxd_assay_dltemplate_view AS
 SELECT DISTINCT s1.sequencenum,
    s1._specimen_key,
    s1.specimenlabel,
    s1._assay_key,
    a1._assaytype_key AS at1,
    a2._assaytype_key AS at2,
    m2._marker_key,
    m2.symbol,
    ea2.accid
   FROM mgd.gxd_assay a1,
    mgd.gxd_specimen s1,
    mgd.gxd_insituresult gs1,
    mgd.gxd_insituresultimage gi1,
    mgd.gxd_assay a2,
    mgd.acc_accession ea2,
    mgd.mrk_marker m2,
    mgd.gxd_specimen s2,
    mgd.gxd_insituresult gs2,
    mgd.gxd_insituresultimage gi2
  WHERE ((a1._assaytype_key = ANY (ARRAY[1, 6, 9])) AND (a1._assay_key = s1._assay_key) AND (s1._specimen_key = gs1._specimen_key) AND (gs1._result_key = gi1._result_key) AND (a2._assay_key = ea2._object_key) AND (ea2._mgitype_key = 8) AND (ea2._logicaldb_key = 1) AND (ea2.preferred = 1) AND (a2._marker_key = m2._marker_key) AND (a2._assaytype_key = ANY (ARRAY[1, 6, 9])) AND (a2._assay_key = s2._assay_key) AND (s2._specimen_key = gs2._specimen_key) AND (gs2._result_key = gi2._result_key) AND (a1._refs_key = a2._refs_key) AND (gi1._imagepane_key = gi2._imagepane_key) AND (a1._assay_key <> ea2._object_key))
  ORDER BY s1.sequencenum, m2.symbol;


CREATE VIEW mgd.gxd_assay_view AS
 SELECT g._assay_key,
    g._assaytype_key,
    g._refs_key,
    g._marker_key,
    g._probeprep_key,
    g._antibodyprep_key,
    g._imagepane_key,
    g._reportergene_key,
    g._createdby_key,
    g._modifiedby_key,
    g.creation_date,
    g.modification_date,
    ac.accid AS mgiid,
    ac.prefixpart,
    ac.numericpart,
    aty.assaytype,
    aty.isrnaassay,
    aty.isgelassay,
    m.symbol,
    m.chromosome,
    m.name,
    c.jnumid,
    c.numericpart AS jnum,
    c.short_citation,
    u1.login AS createdby,
    u2.login AS modifiedby
   FROM mgd.gxd_assay g,
    mgd.acc_accession ac,
    mgd.gxd_assaytype aty,
    mgd.mrk_marker m,
    mgd.bib_citation_cache c,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((g._assay_key = ac._object_key) AND (ac._mgitype_key = 8) AND (ac._logicaldb_key = 1) AND (ac.prefixpart = 'MGI:'::text) AND (ac.preferred = 1) AND (g._assaytype_key = aty._assaytype_key) AND (g._marker_key = m._marker_key) AND (g._refs_key = c._refs_key) AND (g._createdby_key = u1._user_key) AND (g._modifiedby_key = u2._user_key));


CREATE VIEW mgd.gxd_gelband_view AS
 SELECT b._gelband_key,
    b._gellane_key,
    b._gelrow_key,
    b._strength_key,
    b.bandnote,
    b.creation_date,
    b.modification_date,
    s.term AS strength,
    l._assay_key,
    l.sequencenum AS lanenum,
    r.sequencenum AS rownum
   FROM mgd.gxd_gelband b,
    mgd.voc_term s,
    mgd.gxd_gellane l,
    mgd.gxd_gelrow r
  WHERE ((b._strength_key = s._term_key) AND (b._gellane_key = l._gellane_key) AND (b._gelrow_key = r._gelrow_key) AND (r._assay_key = l._assay_key));


CREATE VIEW mgd.gxd_gellane_view AS
 SELECT l._gellane_key,
    l._assay_key,
    l._genotype_key,
    l._gelrnatype_key,
    l._gelcontrol_key,
    l.sequencenum,
    l.lanelabel,
    l.sampleamount,
    l.sex,
    l.age,
    l.agemin,
    l.agemax,
    l.agenote,
    l.lanenote,
    l.creation_date,
    l.modification_date,
    t.term AS rnatype,
    ps.strain,
    c.term AS gellanecontent,
    a.accid AS mgiid
   FROM mgd.gxd_gellane l,
    mgd.voc_term t,
    mgd.voc_term c,
    mgd.gxd_genotype g,
    mgd.prb_strain ps,
    mgd.acc_accession a
  WHERE ((l._gelrnatype_key = t._term_key) AND (l._gelcontrol_key = c._term_key) AND (l._genotype_key = g._genotype_key) AND (g._strain_key = ps._strain_key) AND (g._genotype_key = a._object_key) AND (a._mgitype_key = 12) AND (a._logicaldb_key = 1) AND (a.prefixpart = 'MGI:'::text) AND (a.preferred = 1));


-- Commenting out the view for now since it fails due to faulty parsing by voyager

-- CREATE VIEW mgd.gxd_gellanestructure_view AS
--  SELECT l._assay_key,
--     l.sequencenum,
--     g._gellanestructure_key,
--     g._gellane_key,
--     g._emapa_term_key,
--     g._stage_key,
--     g.creation_date,
--     g.modification_date,
--     s.term,
--     t.stage,
--     ((('TS'::text || ((t.stage)::character varying(5))::text) || ';'::text) || s.term) AS displayit
--    FROM mgd.gxd_gellane l,
--     mgd.gxd_gellanestructure g,
--     mgd.voc_term s,
--     mgd.gxd_theilerstage t
--   WHERE ((l._gellane_key = g._gellane_key) AND (g._emapa_term_key = s._term_key) AND (g._stage_key = t._stage_key));


CREATE VIEW mgd.gxd_gelrow_view AS
 SELECT l._gelrow_key,
    l._assay_key,
    l._gelunits_key,
    l.sequencenum,
    l.size,
    l.rownote,
    l.creation_date,
    l.modification_date,
    to_char(l.size, '9999.99'::text) AS size_str,
    u.term AS units
   FROM mgd.gxd_gelrow l,
    mgd.voc_term u
  WHERE (l._gelunits_key = u._term_key);


CREATE VIEW mgd.gxd_genotype_acc_view AS
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    l.name AS logicaldb,
    s.strain AS description
   FROM mgd.acc_accession a,
    mgd.acc_logicaldb l,
    mgd.gxd_genotype g,
    mgd.prb_strain s
  WHERE ((a._mgitype_key = 12) AND (a.prefixpart = 'MGI:'::text) AND (a._logicaldb_key = l._logicaldb_key) AND (a._object_key = g._genotype_key) AND (g._strain_key = s._strain_key));


CREATE VIEW mgd.gxd_genotype_dataset_view AS
 SELECT DISTINCT v._genotype_key,
    concat(ps.strain, ',', a1.symbol, ',', a2.symbol) AS strain,
    v._refs_key
   FROM mgd.gxd_expression v,
    mgd.gxd_genotype g,
    mgd.prb_strain ps,
    mgd.all_allele a1,
    (mgd.gxd_allelepair ap
     LEFT JOIN mgd.all_allele a2 ON ((ap._allele_key_2 = a2._allele_key)))
  WHERE ((v._genotype_key = g._genotype_key) AND (g._strain_key = ps._strain_key) AND (g._genotype_key = ap._genotype_key) AND (ap._allele_key_1 = a1._allele_key) AND (ap.sequencenum = 1))
UNION ALL
 SELECT DISTINCT t._object_key AS _genotype_key,
    concat(ps.strain, ',', a1.symbol, ',', a2.symbol) AS strain,
    v._refs_key
   FROM mgd.voc_evidence v,
    mgd.voc_annot t,
    mgd.gxd_genotype g,
    mgd.prb_strain ps,
    mgd.all_allele a1,
    (mgd.gxd_allelepair ap
     LEFT JOIN mgd.all_allele a2 ON ((ap._allele_key_2 = a2._allele_key)))
  WHERE ((v._annot_key = t._annot_key) AND (t._annottype_key = ANY (ARRAY[1002, 1020])) AND (t._object_key = g._genotype_key) AND (g._strain_key = ps._strain_key) AND (g._genotype_key = ap._genotype_key) AND (ap._allele_key_1 = a1._allele_key) AND (ap.sequencenum = 1))
UNION ALL
 SELECT DISTINCT v._genotype_key,
    ps.strain,
    v._refs_key
   FROM mgd.gxd_expression v,
    mgd.gxd_genotype g,
    mgd.prb_strain ps
  WHERE ((v._genotype_key = g._genotype_key) AND (g._strain_key = ps._strain_key) AND (NOT (EXISTS ( SELECT 1
           FROM mgd.gxd_allelepair ap
          WHERE (g._genotype_key = ap._genotype_key)))))
UNION ALL
 SELECT DISTINCT t._object_key AS _genotype_key,
    ps.strain,
    v._refs_key
   FROM mgd.voc_evidence v,
    mgd.voc_annot t,
    mgd.gxd_genotype g,
    mgd.prb_strain ps
  WHERE ((v._annot_key = t._annot_key) AND (t._annottype_key = ANY (ARRAY[1002, 1020])) AND (t._object_key = g._genotype_key) AND (g._strain_key = ps._strain_key) AND (NOT (EXISTS ( SELECT 1
           FROM mgd.gxd_allelepair ap
          WHERE (g._genotype_key = ap._genotype_key)))))
  ORDER BY 2;


CREATE VIEW mgd.gxd_genotype_summary_view AS
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    a.accid AS mgiid,
    s.strain AS subtype,
    ((((s.strain || ' '::text) || a1.symbol) || ','::text) || a2.symbol) AS description,
    ((a1.symbol || ','::text) || a2.symbol) AS short_description,
    l.name AS logicaldb
   FROM ((((((mgd.acc_accession a
     JOIN mgd.gxd_genotype g ON ((a._object_key = g._genotype_key)))
     JOIN mgd.prb_strain s ON ((g._strain_key = s._strain_key)))
     JOIN mgd.gxd_allelepair ap ON ((g._genotype_key = ap._genotype_key)))
     JOIN mgd.all_allele a1 ON ((ap._allele_key_1 = a1._allele_key)))
     JOIN mgd.all_allele a2 ON ((ap._allele_key_2 = a2._allele_key)))
     JOIN mgd.acc_logicaldb l ON ((a._logicaldb_key = l._logicaldb_key)))
  WHERE ((a._mgitype_key = 12) AND (a._logicaldb_key = 1) AND (a.prefixpart = 'MGI:'::text) AND (a.preferred = 1) AND (a._logicaldb_key = l._logicaldb_key))
UNION
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    a.accid AS mgiid,
    s.strain AS subtype,
    ((s.strain || ' '::text) || a1.symbol) AS description,
    a1.symbol AS short_description,
    l.name AS logicaldb
   FROM (((((mgd.acc_accession a
     JOIN mgd.gxd_genotype g ON ((a._object_key = g._genotype_key)))
     JOIN mgd.prb_strain s ON ((g._strain_key = s._strain_key)))
     JOIN mgd.gxd_allelepair ap ON (((g._genotype_key = ap._genotype_key) AND (ap._allele_key_2 IS NULL))))
     JOIN mgd.all_allele a1 ON ((ap._allele_key_1 = a1._allele_key)))
     JOIN mgd.acc_logicaldb l ON ((a._logicaldb_key = l._logicaldb_key)))
  WHERE ((a._mgitype_key = 12) AND (a._logicaldb_key = 1) AND (a.prefixpart = 'MGI:'::text) AND (a.preferred = 1) AND (a._logicaldb_key = l._logicaldb_key))
UNION
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    a.accid AS mgiid,
    s.strain AS subtype,
    s.strain AS description,
    NULL::text AS short_description,
    l.name AS logicaldb
   FROM (((mgd.acc_accession a
     JOIN mgd.gxd_genotype g ON ((a._object_key = g._genotype_key)))
     JOIN mgd.prb_strain s ON ((g._strain_key = s._strain_key)))
     JOIN mgd.acc_logicaldb l ON ((a._logicaldb_key = l._logicaldb_key)))
  WHERE ((a._mgitype_key = 12) AND (a._logicaldb_key = 1) AND (a.prefixpart = 'MGI:'::text) AND (a.preferred = 1) AND (a._logicaldb_key = l._logicaldb_key) AND (NOT (EXISTS ( SELECT 1
           FROM mgd.gxd_allelepair ap
          WHERE (g._genotype_key = ap._genotype_key)))));


CREATE VIEW mgd.gxd_genotype_view AS
 SELECT g._genotype_key,
    g._strain_key,
    g.isconditional,
    g.note,
    g._existsas_key,
    g._createdby_key,
    g._modifiedby_key,
    g.creation_date,
    g.modification_date,
    s.strain,
    a.accid AS mgiid,
    ((('['::text || a.accid) || '] '::text) || s.strain) AS displayit,
    u1.login AS createdby,
    u2.login AS modifiedby,
    vt.term AS existsas
   FROM mgd.gxd_genotype g,
    mgd.prb_strain s,
    mgd.acc_accession a,
    mgd.voc_term vt,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((g._strain_key = s._strain_key) AND (g._genotype_key = a._object_key) AND (a._mgitype_key = 12) AND (a._logicaldb_key = 1) AND (a.prefixpart = 'MGI:'::text) AND (a.preferred = 1) AND (g._createdby_key = u1._user_key) AND (g._modifiedby_key = u2._user_key) AND (g._existsas_key = vt._term_key));


CREATE VIEW mgd.gxd_genotypeannotheader_view AS
 SELECT a._annot_key AS annotkey,
    t.term,
    t._term_key AS termkey,
    t.sequencenum AS termsequencenum,
    h.term AS headerterm,
    h._term_key AS headertermkey,
    hh.sequencenum AS headersequencenum
   FROM mgd.voc_annot a,
    mgd.voc_term t,
    mgd.voc_vocabdag vd,
    mgd.dag_node d,
    mgd.dag_closure dc,
    mgd.dag_node dh,
    mgd.voc_term h,
    mgd.voc_annotheader hh
  WHERE ((a._annottype_key = 1002) AND (a._term_key = t._term_key) AND (t._vocab_key = vd._vocab_key) AND (vd._dag_key = d._dag_key) AND (t._term_key = d._object_key) AND (d._node_key = dc._descendent_key) AND (dc._ancestor_key = dh._node_key) AND (dh._label_key = 3) AND (dh._object_key = h._term_key) AND (a._object_key = hh._object_key) AND (h._term_key = hh._term_key))
UNION
 SELECT a._annot_key AS annotkey,
    t.term,
    t._term_key AS termkey,
    t.sequencenum AS termsequencenum,
    h.term AS headerterm,
    h._term_key AS headertermkey,
    hh.sequencenum AS headersequencenum
   FROM mgd.voc_annot a,
    mgd.voc_term t,
    mgd.voc_vocabdag vd,
    mgd.dag_node d,
    mgd.dag_closure dc,
    mgd.dag_node dh,
    mgd.voc_term h,
    mgd.voc_annotheader hh
  WHERE ((a._annottype_key = 1002) AND (a._term_key = t._term_key) AND (t._vocab_key = vd._vocab_key) AND (vd._dag_key = d._dag_key) AND (t._term_key = d._object_key) AND (d._node_key = dc._descendent_key) AND (dc._descendent_key = dh._node_key) AND (dh._label_key = 3) AND (dh._object_key = h._term_key) AND (a._object_key = hh._object_key) AND (h._term_key = hh._term_key));


CREATE VIEW mgd.gxd_index_summaryby_view AS
SELECT
    NULL::integer AS _index_key,
    NULL::text AS markerid,
    NULL::integer AS _marker_key,
    NULL::text AS symbol,
    NULL::text AS name,
    NULL::text AS markerstatus,
    NULL::text AS markertype,
    NULL::integer AS _indexassay_key,
    NULL::text AS indexassay,
    NULL::integer AS sequencenum,
    NULL::text AS age,
    NULL::text AS priority,
    NULL::text AS conditional,
    NULL::integer AS isfullcoded,
    NULL::text AS jnumid,
    NULL::text AS short_citation,
    NULL::text AS comments,
    NULL::text AS synonyms;


CREATE VIEW mgd.gxd_index_view AS
 SELECT i._index_key,
    i._refs_key,
    i._marker_key,
    i._priority_key,
    i._conditionalmutants_key,
    i.comments,
    i._createdby_key,
    i._modifiedby_key,
    i.creation_date,
    i.modification_date,
    m.symbol,
    c.jnumid,
    c.numericpart AS jnum,
    c.short_citation,
    u1.login AS createdby,
    u2.login AS modifiedby
   FROM mgd.gxd_index i,
    mgd.mrk_marker m,
    mgd.bib_citation_cache c,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((i._marker_key = m._marker_key) AND (i._refs_key = c._refs_key) AND (i._createdby_key = u1._user_key) AND (i._modifiedby_key = u2._user_key));


CREATE VIEW mgd.gxd_insituresult_view AS
 SELECT r._result_key,
    r._specimen_key,
    r._strength_key,
    r._pattern_key,
    r.sequencenum,
    r.resultnote,
    r.creation_date,
    r.modification_date,
    s.term AS strength,
    p.term AS pattern
   FROM mgd.gxd_insituresult r,
    mgd.voc_term s,
    mgd.voc_term p
  WHERE ((r._strength_key = s._term_key) AND (r._pattern_key = p._term_key));


CREATE VIEW mgd.gxd_isresultcelltype_view AS
 SELECT gs._assay_key,
    r._specimen_key,
    r.sequencenum,
    g._resultcelltype_key,
    g._result_key,
    g._celltype_term_key,
    g.creation_date,
    g.modification_date,
    s.term,
    s.term AS displayit
   FROM mgd.gxd_insituresult r,
    mgd.gxd_isresultcelltype g,
    mgd.voc_term s,
    mgd.gxd_specimen gs
  WHERE ((r._result_key = g._result_key) AND (g._celltype_term_key = s._term_key) AND (r._specimen_key = gs._specimen_key));


CREATE VIEW mgd.gxd_isresultimage_view AS
 SELECT r._specimen_key,
    r.sequencenum,
    i._resultimage_key,
    i._result_key,
    i._imagepane_key,
    i.creation_date,
    i.modification_date,
    concat(m.figurelabel, p.panelabel) AS figurepanelabel,
    p._image_key,
    p.panelabel,
    p.x,
    p.y,
    p.width,
    p.height,
    m.figurelabel,
    m.xdim,
    m.ydim,
    a.accid,
    NULL::integer AS pixid
   FROM mgd.gxd_insituresult r,
    mgd.gxd_insituresultimage i,
    mgd.img_imagepane p,
    mgd.img_image m,
    mgd.acc_accession a
  WHERE ((r._result_key = i._result_key) AND (i._imagepane_key = p._imagepane_key) AND (p._image_key = m._image_key) AND (p._image_key = a._object_key) AND (a._logicaldb_key = 1) AND (a._mgitype_key = 9) AND (NOT (EXISTS ( SELECT
           FROM mgd.acc_accession aa
          WHERE ((p._image_key = aa._object_key) AND (aa._mgitype_key = 9) AND (aa._logicaldb_key = 19))))))
UNION
 SELECT r._specimen_key,
    r.sequencenum,
    i._resultimage_key,
    i._result_key,
    i._imagepane_key,
    i.creation_date,
    i.modification_date,
    concat(m.figurelabel, p.panelabel) AS figurepanelabel,
    p._image_key,
    p.panelabel,
    p.x,
    p.y,
    p.width,
    p.height,
    m.figurelabel,
    m.xdim,
    m.ydim,
    a.accid,
    aa.numericpart AS pixid
   FROM mgd.gxd_insituresult r,
    mgd.gxd_insituresultimage i,
    mgd.img_imagepane p,
    mgd.img_image m,
    mgd.acc_accession a,
    mgd.acc_accession aa
  WHERE ((r._result_key = i._result_key) AND (i._imagepane_key = p._imagepane_key) AND (p._image_key = m._image_key) AND (p._image_key = a._object_key) AND (a._logicaldb_key = 1) AND (a._mgitype_key = 9) AND (p._image_key = aa._object_key) AND (aa._logicaldb_key = 19) AND (aa._mgitype_key = 9));


-- Commenting out the view for now since it fails due to faulty parsing by voyager

-- CREATE VIEW mgd.gxd_isresultstructure_view AS
--  SELECT r._specimen_key,
--     r.sequencenum,
--     g._resultstructure_key,
--     g._result_key,
--     g._emapa_term_key,
--     g._stage_key,
--     g.creation_date,
--     g.modification_date,
--     s.term,
--     t.stage,
--     ((('TS'::text || ((t.stage)::character varying(5))::text) || ';'::text) || s.term) AS displayit
--    FROM mgd.gxd_insituresult r,
--     mgd.gxd_isresultstructure g,
--     mgd.voc_term s,
--     mgd.gxd_theilerstage t
--   WHERE ((r._result_key = g._result_key) AND (g._emapa_term_key = s._term_key) AND (g._stage_key = t._stage_key));


CREATE VIEW mgd.gxd_probeprep_view AS
 SELECT a._assay_key,
    pp._probeprep_key,
    pp._probe_key,
    pp._sense_key,
    pp._label_key,
    pp._visualization_key,
    pp.type,
    pp.creation_date,
    pp.modification_date,
    s.term AS sense,
    l.term AS label,
    v.term AS visualization,
    p.name AS probename,
    ac.accid
   FROM mgd.gxd_assay a,
    mgd.gxd_probeprep pp,
    mgd.voc_term s,
    mgd.voc_term l,
    mgd.voc_term v,
    mgd.prb_probe p,
    mgd.acc_accession ac
  WHERE ((a._probeprep_key = pp._probeprep_key) AND (pp._sense_key = s._term_key) AND (pp._label_key = l._term_key) AND (pp._visualization_key = v._term_key) AND (pp._probe_key = p._probe_key) AND (p._probe_key = ac._object_key) AND (ac._mgitype_key = 3) AND (ac._logicaldb_key = 1) AND (ac.prefixpart = 'MGI:'::text) AND (ac.preferred = 1));


CREATE VIEW mgd.gxd_specimen_view AS
 SELECT s._specimen_key,
    s._assay_key,
    s._embedding_key,
    s._fixation_key,
    s._genotype_key,
    s.sequencenum,
    s.specimenlabel,
    s.sex,
    s.age,
    s.agemin,
    s.agemax,
    s.agenote,
    s.hybridization,
    s.specimennote,
    s.creation_date,
    s.modification_date,
    e.term AS embeddingmethod,
    f.term AS fixation,
    ps.strain,
    a.accid AS mgiid
   FROM mgd.gxd_specimen s,
    mgd.voc_term e,
    mgd.voc_term f,
    mgd.gxd_genotype g,
    mgd.prb_strain ps,
    mgd.acc_accession a
  WHERE ((s._embedding_key = e._term_key) AND (s._fixation_key = f._term_key) AND (s._genotype_key = g._genotype_key) AND (g._strain_key = ps._strain_key) AND (s._genotype_key = a._object_key) AND (a._mgitype_key = 12) AND (a._logicaldb_key = 1) AND (a.prefixpart = 'MGI:'::text) AND (a.preferred = 1));


CREATE VIEW mgd.img_image_acc_view AS
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    l.name AS logicaldb
   FROM mgd.acc_accession a,
    mgd.acc_logicaldb l
  WHERE ((a._mgitype_key = 9) AND (a._logicaldb_key = l._logicaldb_key));


CREATE VIEW mgd.img_image_summary2_view AS
 SELECT p._imagepane_key,
    concat(m.figurelabel, p.panelabel) AS figurepanelabel,
    p._image_key,
    p.panelabel,
    p.x,
    p.y,
    p.width,
    p.height,
    m.figurelabel,
    m.xdim,
    m.ydim,
    a.accid,
    NULL::integer AS pixid
   FROM mgd.img_imagepane p,
    mgd.img_image m,
    mgd.acc_accession a
  WHERE ((p._image_key = m._image_key) AND (p._image_key = a._object_key) AND (a._logicaldb_key = 1) AND (a._mgitype_key = 9) AND (NOT (EXISTS ( SELECT
           FROM mgd.acc_accession aa
          WHERE ((p._image_key = aa._object_key) AND (aa._mgitype_key = 9) AND (aa._logicaldb_key = 19))))))
UNION
 SELECT p._imagepane_key,
    concat(m.figurelabel, p.panelabel) AS figurepanelabel,
    p._image_key,
    p.panelabel,
    p.x,
    p.y,
    p.width,
    p.height,
    m.figurelabel,
    m.xdim,
    m.ydim,
    a.accid,
    aa.numericpart AS pixid
   FROM mgd.img_imagepane p,
    mgd.img_image m,
    mgd.acc_accession a,
    mgd.acc_accession aa
  WHERE ((p._image_key = m._image_key) AND (p._image_key = a._object_key) AND (a._logicaldb_key = 1) AND (a._mgitype_key = 9) AND (p._image_key = aa._object_key) AND (aa._logicaldb_key = 19) AND (aa._mgitype_key = 9));


CREATE VIEW mgd.img_image_summarybyreference_view AS
 SELECT DISTINCT i._refs_key,
    c.jnumid,
    p._imagepane_key,
    i.figurelabel,
    i.xdim,
    i.ydim,
    p.panelabel,
    p.x,
    p.y,
    p.width,
    p.height,
    s.specimenlabel,
    s.specimennote,
    a1.accid AS imageid,
    a2.accid AS pixid,
    a3.accid AS assayid,
    a4.accid AS markerid,
    m.symbol,
    t.assaytype
   FROM mgd.bib_citation_cache c,
    mgd.img_imagepane p,
    mgd.gxd_insituresultimage gri,
    mgd.gxd_insituresult gr,
    mgd.gxd_specimen s,
    mgd.gxd_assay a,
    mgd.gxd_assaytype t,
    mgd.acc_accession a1,
    mgd.acc_accession a3,
    mgd.acc_accession a4,
    mgd.mrk_marker m,
    (mgd.img_image i
     LEFT JOIN mgd.acc_accession a2 ON (((i._image_key = a2._object_key) AND (i._image_key = a2._object_key) AND (a2._mgitype_key = 9) AND (a2._logicaldb_key = 19))))
  WHERE ((c._refs_key = i._refs_key) AND (i._imageclass_key = 6481781) AND (i._imagetype_key = 1072158) AND (i._image_key = p._image_key) AND (p._imagepane_key = gri._imagepane_key) AND (gri._result_key = gr._result_key) AND (gr._specimen_key = s._specimen_key) AND (s._assay_key = a._assay_key) AND (a._assaytype_key = t._assaytype_key) AND (i._image_key = a1._object_key) AND (a1._mgitype_key = 9) AND (a1._logicaldb_key = 1) AND (a._assay_key = a3._object_key) AND (a3._mgitype_key = 8) AND (a3._logicaldb_key = 1) AND (a._marker_key = a4._object_key) AND (a4._mgitype_key = 2) AND (a4._logicaldb_key = 1) AND (a4.preferred = 1) AND (a._marker_key = m._marker_key))
UNION
 SELECT DISTINCT i._refs_key,
    c.jnumid,
    p._imagepane_key,
    i.figurelabel,
    i.xdim,
    i.ydim,
    p.panelabel,
    p.x,
    p.y,
    p.width,
    p.height,
    NULL::text AS specimenlabel,
    NULL::text AS specimennote,
    a1.accid AS imageid,
    a2.accid AS pixid,
    a3.accid AS assayid,
    a4.accid AS markerid,
    m.symbol,
    t.assaytype
   FROM mgd.bib_citation_cache c,
    mgd.img_imagepane p,
    mgd.gxd_assay a,
    mgd.gxd_assaytype t,
    mgd.acc_accession a1,
    mgd.acc_accession a3,
    mgd.acc_accession a4,
    mgd.mrk_marker m,
    (mgd.img_image i
     LEFT JOIN mgd.acc_accession a2 ON (((i._image_key = a2._object_key) AND (i._image_key = a2._object_key) AND (a2._mgitype_key = 9) AND (a2._logicaldb_key = 19))))
  WHERE ((c._refs_key = i._refs_key) AND (i._imageclass_key = 6481781) AND (i._imagetype_key = 1072158) AND (i._image_key = p._image_key) AND (p._imagepane_key = a._imagepane_key) AND (a._assaytype_key = t._assaytype_key) AND (i._image_key = a1._object_key) AND (a1._mgitype_key = 9) AND (a1._logicaldb_key = 1) AND (a._assay_key = a3._object_key) AND (a3._mgitype_key = 8) AND (a3._logicaldb_key = 1) AND (a._marker_key = a4._object_key) AND (a4._mgitype_key = 2) AND (a4._logicaldb_key = 1) AND (a4.preferred = 1) AND (a._marker_key = m._marker_key))
UNION
 SELECT DISTINCT i._refs_key,
    c.jnumid,
    p._imagepane_key,
    i.figurelabel,
    i.xdim,
    i.ydim,
    p.panelabel,
    p.x,
    p.y,
    p.width,
    p.height,
    NULL::text AS specimenlabel,
    NULL::text AS specimennote,
    a1.accid AS imageid,
    a2.accid AS pixid,
    NULL::text AS assayid,
    NULL::text AS markerid,
    NULL::text AS symbol,
    NULL::text AS assaytype
   FROM mgd.bib_citation_cache c,
    mgd.img_imagepane p,
    mgd.acc_accession a1,
    (mgd.img_image i
     LEFT JOIN mgd.acc_accession a2 ON (((i._image_key = a2._object_key) AND (i._image_key = a2._object_key) AND (a2._mgitype_key = 9) AND (a2._logicaldb_key = 19))))
  WHERE ((c._refs_key = i._refs_key) AND (i._imageclass_key = 6481781) AND (i._imagetype_key = 1072158) AND (i._image_key = p._image_key) AND (NOT (EXISTS ( SELECT 1
           FROM mgd.gxd_assay a,
            mgd.gxd_specimen s,
            mgd.gxd_insituresult gr,
            mgd.gxd_insituresultimage gri
          WHERE ((i._refs_key = a._refs_key) AND (a._assay_key = s._assay_key) AND (s._specimen_key = gr._specimen_key) AND (gr._result_key = gri._result_key) AND (gri._imagepane_key = p._imagepane_key))))) AND (NOT (EXISTS ( SELECT 1
           FROM mgd.gxd_assay a
          WHERE ((i._refs_key = a._refs_key) AND (a._imagepane_key = p._imagepane_key))))) AND (i._image_key = a1._object_key) AND (a1._mgitype_key = 9) AND (a1._logicaldb_key = 1))
  ORDER BY 4, 7;


CREATE VIEW mgd.img_image_view AS
 SELECT i._image_key,
    i._imageclass_key,
    i._imagetype_key,
    i._refs_key,
    i._thumbnailimage_key,
    i.xdim,
    i.ydim,
    i.figurelabel,
    i._createdby_key,
    i._modifiedby_key,
    i.creation_date,
    i.modification_date,
    ia.accid AS mgiid,
    ia.prefixpart,
    ia.numericpart,
    t1.term AS imagetype,
    t2.term AS imageclass,
    c.jnumid,
    c.numericpart AS jnum,
    c.short_citation,
    u1.login AS createdby,
    u2.login AS modifiedb
   FROM mgd.img_image i,
    mgd.acc_accession ia,
    mgd.acc_mgitype m,
    mgd.bib_citation_cache c,
    mgd.voc_term t1,
    mgd.voc_term t2,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((i._image_key = ia._object_key) AND (ia._mgitype_key = 9) AND (ia._logicaldb_key = 1) AND (ia.prefixpart = 'MGI:'::text) AND (ia.preferred = 1) AND (i._imagetype_key = t1._term_key) AND (i._imageclass_key = t2._term_key) AND (i._refs_key = c._refs_key) AND (i._createdby_key = u1._user_key) AND (i._modifiedby_key = u2._user_key));


CREATE VIEW mgd.img_imagepane_assoc_view AS
 SELECT ip._assoc_key,
    ip._imagepane_key,
    ip._mgitype_key,
    ip._object_key,
    ip.isprimary,
    ip._createdby_key,
    ip._modifiedby_key,
    ip.creation_date,
    ip.modification_date,
    "substring"(i.figurelabel, 1, 20) AS figurelabel,
    i._imageclass_key,
    t.term,
    a1.accid AS mgiid,
    a2.accid AS pixid,
    i._image_key
   FROM mgd.img_imagepane_assoc ip,
    mgd.img_imagepane p,
    mgd.img_image i,
    mgd.voc_term t,
    mgd.acc_accession a1,
    mgd.acc_accession a2
  WHERE ((ip._imagepane_key = p._imagepane_key) AND (p._image_key = i._image_key) AND (i._imageclass_key = t._term_key) AND (p._image_key = a1._object_key) AND (a1._mgitype_key = 9) AND (a1._logicaldb_key = 1) AND (a1.prefixpart = 'MGI:'::text) AND (a1.preferred = 1) AND (p._image_key = a2._object_key) AND (a2._mgitype_key = 9) AND (a2._logicaldb_key = 19))
UNION
 SELECT ip._assoc_key,
    ip._imagepane_key,
    ip._mgitype_key,
    ip._object_key,
    ip.isprimary,
    ip._createdby_key,
    ip._modifiedby_key,
    ip.creation_date,
    ip.modification_date,
    "substring"(i.figurelabel, 1, 20) AS figurelabel,
    i._imageclass_key,
    t.term,
    a1.accid AS mgiid,
    NULL::text AS pixid,
    i._image_key
   FROM mgd.img_imagepane_assoc ip,
    mgd.img_imagepane p,
    mgd.img_image i,
    mgd.voc_term t,
    mgd.acc_accession a1
  WHERE ((ip._imagepane_key = p._imagepane_key) AND (p._image_key = i._image_key) AND (i._imageclass_key = t._term_key) AND (p._image_key = a1._object_key) AND (a1._mgitype_key = 9) AND (a1._logicaldb_key = 1) AND (a1.prefixpart = 'MGI:'::text) AND (a1.preferred = 1) AND (NOT (EXISTS ( SELECT 1
           FROM mgd.acc_accession a2
          WHERE ((p._image_key = a2._object_key) AND (a2._mgitype_key = 9) AND (a2._logicaldb_key = 19))))));


CREATE VIEW mgd.img_imagepaneallele_view AS
 SELECT DISTINCT ipall._assoc_key,
    a._allele_key,
    i.figurelabel,
    ipall._imagepane_key,
    assoc.symbol,
    aa.accid AS alleleid
   FROM mgd.all_allele a,
    mgd.img_image i,
    mgd.img_imagepane ip,
    mgd.img_imagepane_assoc ipa,
    mgd.img_imagepane_assoc ipall,
    mgd.all_allele assoc,
    mgd.acc_accession aa
  WHERE ((a._allele_key = ipa._object_key) AND (i._image_key = ip._image_key) AND (ip._imagepane_key = ipa._imagepane_key) AND (ipa._mgitype_key = 11) AND (i._imagetype_key = 1072158) AND (ipa._imagepane_key = ipall._imagepane_key) AND (ipall._mgitype_key = 11) AND (ipall._object_key = assoc._allele_key) AND (ipall._object_key = aa._object_key) AND (aa._mgitype_key = 11))
  ORDER BY i.figurelabel, assoc.symbol;


CREATE VIEW mgd.img_imagepanegenotype_view AS
 SELECT DISTINCT ipa._assoc_key,
    ipa._imagepane_key,
    ipav._allele_key,
    ipav.figurelabel,
    s.strain,
    n.note AS allelecomposition,
    a.accid
   FROM mgd.img_imagepane_assoc ipa,
    mgd.gxd_genotype g,
    mgd.prb_strain s,
    mgd.mgi_note n,
    mgd.img_imagepaneallele_view ipav,
    mgd.acc_accession a
  WHERE ((ipa._mgitype_key = 12) AND (ipa._object_key = g._genotype_key) AND (g._strain_key = s._strain_key) AND (g._genotype_key = n._object_key) AND (n._notetype_key = 1016) AND (n._mgitype_key = 12) AND (ipa._imagepane_key = ipav._imagepane_key) AND (g._genotype_key = a._object_key) AND (a._mgitype_key = 12) AND (a._logicaldb_key = 1));


CREATE VIEW mgd.img_imagepanegxd_view AS
 SELECT r._image_key,
    r._refs_key,
    i._imagepane_key,
    concat(r.figurelabel, i.panelabel) AS panelabel,
    c.jnumid,
    c.jnum,
    c.short_citation
   FROM mgd.img_image r,
    mgd.img_imagepane i,
    mgd.bib_all_view c
  WHERE ((r._imageclass_key = 6481781) AND (r._image_key = i._image_key) AND (r._imagetype_key = 1072158) AND (r._refs_key = c._refs_key));


CREATE VIEW mgd.map_gm_coord_feature_view AS
 SELECT f._feature_key,
    c.name AS collectionname,
    a1.accid AS seqid,
    ch._chromosome_key,
    ch.chromosome AS genomicchromosome,
    f._object_key AS _sequence_key,
    f.startcoordinate,
    f.endcoordinate,
    f.strand,
    f._createdby_key,
    f._modifiedby_key,
    f.creation_date,
    f.modification_date
   FROM mgd.map_coord_collection c,
    mgd.map_coordinate mc,
    mgd.map_coord_feature f,
    mgd.mrk_chromosome ch,
    mgd.acc_accession a1
  WHERE ((c.name = ANY (ARRAY['NCBI Gene Model'::text, 'VISTA Gene Model'::text, 'Ensembl Reg Gene Model'::text, 'Ensembl Gene Model'::text])) AND (c._collection_key = mc._collection_key) AND (mc._object_key = ch._chromosome_key) AND (mc._map_key = f._map_key) AND (f._object_key = a1._object_key) AND (a1._mgitype_key = 19) AND (a1.preferred = 1))
  ORDER BY c._collection_key, f._feature_key;


CREATE VIEW mgd.mgi_note_allele_view AS
 SELECT n._note_key,
    n._object_key,
    n._mgitype_key,
    n._notetype_key,
    n.note,
    n._createdby_key,
    n._modifiedby_key,
    n.creation_date,
    n.modification_date,
    t.notetype,
    m.name AS mgitype,
    u1.login AS createdby,
    u2.login AS modifiedby
   FROM mgd.mgi_note n,
    mgd.mgi_notetype t,
    mgd.acc_mgitype m,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((n._notetype_key = t._notetype_key) AND (t._mgitype_key = 11) AND (n._mgitype_key = m._mgitype_key) AND (n._createdby_key = u1._user_key) AND (n._modifiedby_key = u2._user_key));


CREATE VIEW mgd.mgi_note_allelevariant_view AS
 SELECT n._note_key,
    n._object_key,
    n._mgitype_key,
    n._notetype_key,
    n.note,
    n._createdby_key,
    n._modifiedby_key,
    n.creation_date,
    n.modification_date,
    t.notetype,
    m.name AS mgitype,
    u1.login AS createdby,
    u2.login AS modifiedby
   FROM mgd.mgi_note n,
    mgd.mgi_notetype t,
    mgd.acc_mgitype m,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((n._notetype_key = t._notetype_key) AND (t._mgitype_key = 45) AND (n._mgitype_key = m._mgitype_key) AND (n._createdby_key = u1._user_key) AND (n._modifiedby_key = u2._user_key));


CREATE VIEW mgd.mgi_note_derivation_view AS
 SELECT n._note_key,
    n._object_key,
    n._mgitype_key,
    n._notetype_key,
    n.note,
    n._createdby_key,
    n._modifiedby_key,
    n.creation_date,
    n.modification_date,
    t.notetype,
    m.name AS mgitype,
    u1.login AS createdby,
    u2.login AS modifiedby
   FROM mgd.mgi_note n,
    mgd.mgi_notetype t,
    mgd.acc_mgitype m,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((n._notetype_key = t._notetype_key) AND (t._mgitype_key = 36) AND (n._mgitype_key = m._mgitype_key) AND (n._createdby_key = u1._user_key) AND (n._modifiedby_key = u2._user_key));


CREATE VIEW mgd.mgi_note_genotype_view AS
 SELECT n._note_key,
    n._object_key,
    n._mgitype_key,
    n._notetype_key,
    n.note,
    n._createdby_key,
    n._modifiedby_key,
    n.creation_date,
    n.modification_date,
    t.notetype,
    m.name AS mgitype,
    u1.login AS createdby,
    u2.login AS modifiedby
   FROM mgd.mgi_note n,
    mgd.mgi_notetype t,
    mgd.acc_mgitype m,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((n._notetype_key = t._notetype_key) AND (t._mgitype_key = 12) AND (n._mgitype_key = m._mgitype_key) AND (n._createdby_key = u1._user_key) AND (n._modifiedby_key = u2._user_key));


CREATE VIEW mgd.mgi_note_image_view AS
 SELECT n._note_key,
    n._object_key,
    n._mgitype_key,
    n._notetype_key,
    n.note,
    n._createdby_key,
    n._modifiedby_key,
    n.creation_date,
    n.modification_date,
    t.notetype,
    m.name AS mgitype,
    u1.login AS createdby,
    u2.login AS modifiedby
   FROM mgd.mgi_note n,
    mgd.mgi_notetype t,
    mgd.acc_mgitype m,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((n._notetype_key = t._notetype_key) AND (t._mgitype_key = 9) AND (n._mgitype_key = m._mgitype_key) AND (n._createdby_key = u1._user_key) AND (n._modifiedby_key = u2._user_key));


CREATE VIEW mgd.mgi_note_marker_view AS
 SELECT n._note_key,
    n._object_key,
    n._mgitype_key,
    n._notetype_key,
    n.note,
    n._createdby_key,
    n._modifiedby_key,
    n.creation_date,
    n.modification_date,
    t.notetype,
    m.name AS mgitype,
    u1.login AS createdby,
    u2.login AS modifiedby
   FROM mgd.mgi_note n,
    mgd.mgi_notetype t,
    mgd.acc_mgitype m,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((n._notetype_key = t._notetype_key) AND (t._mgitype_key = 2) AND (n._mgitype_key = m._mgitype_key) AND (n._createdby_key = u1._user_key) AND (n._modifiedby_key = u2._user_key));


CREATE VIEW mgd.mgi_note_probe_view AS
 SELECT n._note_key,
    n._object_key,
    n._mgitype_key,
    n._notetype_key,
    n.note,
    n._createdby_key,
    n._modifiedby_key,
    n.creation_date,
    n.modification_date,
    t.notetype,
    m.name AS mgitype,
    u1.login AS createdby,
    u2.login AS modifiedby
   FROM mgd.mgi_note n,
    mgd.mgi_notetype t,
    mgd.acc_mgitype m,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((n._notetype_key = t._notetype_key) AND (t._mgitype_key = 3) AND (n._mgitype_key = m._mgitype_key) AND (n._createdby_key = u1._user_key) AND (n._modifiedby_key = u2._user_key));


CREATE VIEW mgd.mgi_note_strain_view AS
 SELECT n._note_key,
    n._object_key,
    n._mgitype_key,
    n._notetype_key,
    n.note,
    n._createdby_key,
    n._modifiedby_key,
    n.creation_date,
    n.modification_date,
    t.notetype,
    m.name AS mgitype,
    u1.login AS createdby,
    u2.login AS modifiedby
   FROM mgd.mgi_note n,
    mgd.mgi_notetype t,
    mgd.acc_mgitype m,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((n._notetype_key = t._notetype_key) AND (t._mgitype_key = 10) AND (n._mgitype_key = m._mgitype_key) AND (n._createdby_key = u1._user_key) AND (n._modifiedby_key = u2._user_key));


CREATE VIEW mgd.mgi_note_vocevidence_view AS
 SELECT n._note_key,
    n._object_key,
    n._mgitype_key,
    n._notetype_key,
    n.note,
    n._createdby_key,
    n._modifiedby_key,
    n.creation_date,
    n.modification_date,
    t.notetype,
    m.name AS mgitype,
    u1.login AS createdby,
    u2.login AS modifiedby
   FROM mgd.mgi_note n,
    mgd.mgi_notetype t,
    mgd.acc_mgitype m,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((n._notetype_key = t._notetype_key) AND (t._mgitype_key = 25) AND (n._mgitype_key = m._mgitype_key) AND (n._createdby_key = u1._user_key) AND (n._modifiedby_key = u2._user_key));


CREATE VIEW mgd.mgi_notetype_strain_view AS
 SELECT mgi_notetype._notetype_key,
    mgi_notetype._mgitype_key,
    mgi_notetype.notetype,
    mgi_notetype.private,
    mgi_notetype._createdby_key,
    mgi_notetype._modifiedby_key,
    mgi_notetype.creation_date,
    mgi_notetype.modification_date
   FROM mgd.mgi_notetype
  WHERE (mgi_notetype._mgitype_key = 10);


CREATE VIEW mgd.mgi_organism_allele_view AS
 SELECT s._organism_key,
    s.commonname,
    s.latinname,
    s._createdby_key,
    s._modifiedby_key,
    s.creation_date,
    s.modification_date,
    (((s.commonname || ' ('::text) || s.latinname) || ')'::text) AS organism
   FROM mgd.mgi_organism s,
    mgd.mgi_organism_mgitype t
  WHERE ((s._organism_key = t._organism_key) AND (t._mgitype_key = 11));


CREATE VIEW mgd.mgi_organism_antigen_view AS
 SELECT s._organism_key,
    s.commonname,
    s.latinname,
    s._createdby_key,
    s._modifiedby_key,
    s.creation_date,
    s.modification_date
   FROM mgd.mgi_organism s,
    mgd.mgi_organism_mgitype t
  WHERE ((s._organism_key = t._organism_key) AND (t._mgitype_key = 7));


CREATE VIEW mgd.mgi_organism_marker_view AS
 SELECT s._organism_key,
    s.commonname,
    s.latinname,
    s._createdby_key,
    s._modifiedby_key,
    s.creation_date,
    s.modification_date,
    (((s.commonname || ' ('::text) || s.latinname) || ')'::text) AS organism
   FROM mgd.mgi_organism s,
    mgd.mgi_organism_mgitype t
  WHERE ((s._organism_key = t._organism_key) AND (t._mgitype_key = 2));


CREATE VIEW mgd.mgi_organism_mgitype_view AS
 SELECT st._assoc_key,
    st._organism_key,
    st._mgitype_key,
    st.sequencenum,
    st._createdby_key,
    st._modifiedby_key,
    st.creation_date,
    st.modification_date,
    s.commonname,
    s.latinname,
    t.name AS typename
   FROM mgd.mgi_organism_mgitype st,
    mgd.mgi_organism s,
    mgd.acc_mgitype t
  WHERE ((st._organism_key = s._organism_key) AND (st._mgitype_key = t._mgitype_key));


CREATE VIEW mgd.mgi_organism_probe_view AS
 SELECT s._organism_key,
    s.commonname,
    s.latinname,
    s._createdby_key,
    s._modifiedby_key,
    s.creation_date,
    s.modification_date
   FROM mgd.mgi_organism s,
    mgd.mgi_organism_mgitype t
  WHERE ((s._organism_key = t._organism_key) AND (t._mgitype_key = 3));


CREATE VIEW mgd.mgi_reference_allele_view AS
 SELECT r._assoc_key,
    r._refs_key,
    r._object_key,
    r._mgitype_key,
    r._refassoctype_key,
    r._createdby_key,
    r._modifiedby_key,
    r.creation_date,
    r.modification_date,
    t.assoctype,
    t.allowonlyone,
    c.jnumid,
    c.numericpart AS jnum,
    c.short_citation,
    c.isreviewarticle,
    c.isreviewarticlestring,
    a.symbol,
    aa.accid,
    m.symbol AS markersymbol
   FROM mgd.mgi_reference_assoc r,
    mgd.mgi_refassoctype t,
    mgd.all_allele a,
    mgd.acc_accession aa,
    mgd.mrk_marker m,
    mgd.bib_citation_cache c
  WHERE ((r._mgitype_key = 11) AND (r._refassoctype_key = t._refassoctype_key) AND (r._refs_key = c._refs_key) AND (r._object_key = a._allele_key) AND (a._allele_key = aa._object_key) AND (aa._mgitype_key = 11) AND (aa._logicaldb_key = 1) AND (aa.preferred = 1) AND (a._marker_key = m._marker_key))
UNION
 SELECT r._assoc_key,
    r._refs_key,
    r._object_key,
    r._mgitype_key,
    r._refassoctype_key,
    r._createdby_key,
    r._modifiedby_key,
    r.creation_date,
    r.modification_date,
    t.assoctype,
    t.allowonlyone,
    c.jnumid,
    c.numericpart AS jnum,
    c.short_citation,
    c.isreviewarticle,
    c.isreviewarticlestring,
    a.symbol,
    aa.accid,
    NULL::text AS markersymbol
   FROM mgd.mgi_reference_assoc r,
    mgd.mgi_refassoctype t,
    mgd.all_allele a,
    mgd.acc_accession aa,
    mgd.bib_citation_cache c
  WHERE ((r._mgitype_key = 11) AND (r._refassoctype_key = t._refassoctype_key) AND (r._refs_key = c._refs_key) AND (r._object_key = a._allele_key) AND (a._allele_key = aa._object_key) AND (aa._mgitype_key = 11) AND (aa._logicaldb_key = 1) AND (aa.preferred = 1) AND (a._marker_key IS NULL))
  ORDER BY 17, 10;


CREATE VIEW mgd.mgi_reference_allelevariant_view AS
 SELECT r._assoc_key,
    r._refs_key,
    r._object_key,
    r._mgitype_key,
    r._refassoctype_key,
    r._createdby_key,
    r._modifiedby_key,
    r.creation_date,
    r.modification_date,
    t.assoctype,
    t.allowonlyone,
    c.jnumid,
    c.numericpart AS jnum,
    c.short_citation,
    c.isreviewarticle,
    c.isreviewarticlestring
   FROM mgd.mgi_reference_assoc r,
    mgd.mgi_refassoctype t,
    mgd.bib_citation_cache c
  WHERE ((r._mgitype_key = 45) AND (r._refassoctype_key = t._refassoctype_key) AND (r._refs_key = c._refs_key));


CREATE VIEW mgd.mgi_reference_antibody_view AS
 SELECT r._assoc_key,
    r._refs_key,
    r._object_key,
    r._mgitype_key,
    r._refassoctype_key,
    r._createdby_key,
    r._modifiedby_key,
    r.creation_date,
    r.modification_date,
    t.assoctype,
    t.allowonlyone,
    c.jnumid,
    c.numericpart AS jnum,
    c.short_citation,
    c.isreviewarticle,
    c.isreviewarticlestring
   FROM mgd.mgi_reference_assoc r,
    mgd.mgi_refassoctype t,
    mgd.bib_citation_cache c
  WHERE ((r._mgitype_key = 6) AND (r._refassoctype_key = t._refassoctype_key) AND (r._refs_key = c._refs_key));


CREATE VIEW mgd.mgi_reference_doid_view AS
 SELECT r._assoc_key,
    r._refs_key,
    r._object_key,
    r._mgitype_key,
    r._refassoctype_key,
    r._createdby_key,
    r._modifiedby_key,
    r.creation_date,
    r.modification_date,
    t.assoctype,
    t.allowonlyone,
    c.jnumid,
    c.numericpart AS jnum,
    c.short_citation,
    c.isreviewarticle,
    c.isreviewarticlestring,
    u.login AS modifiedby,
    vt.term,
    aa.accid
   FROM mgd.mgi_reference_assoc r,
    mgd.mgi_refassoctype t,
    mgd.bib_citation_cache c,
    mgd.mgi_user u,
    mgd.voc_term vt,
    mgd.acc_accession aa
  WHERE ((r._mgitype_key = 13) AND (r._refassoctype_key = 1032) AND (r._refassoctype_key = t._refassoctype_key) AND (r._refs_key = c._refs_key) AND (r._modifiedby_key = u._user_key) AND (r._object_key = vt._term_key) AND (vt._term_key = aa._object_key) AND (aa._mgitype_key = 13) AND (aa._logicaldb_key = 191) AND (aa.preferred = 1))
  ORDER BY vt.term, t.assoctype;


CREATE VIEW mgd.mgi_reference_marker_view AS
 SELECT r._assoc_key,
    r._refs_key,
    r._object_key,
    r._mgitype_key,
    r._refassoctype_key,
    r._createdby_key,
    r._modifiedby_key,
    r.creation_date,
    r.modification_date,
    t.assoctype,
    t.allowonlyone,
    c.jnumid,
    c.numericpart AS jnum,
    c.short_citation,
    c.isreviewarticle,
    c.isreviewarticlestring,
    u2.login AS modifiedby,
    u1.login AS createdby,
    m.symbol,
    aa.accid
   FROM mgd.mgi_reference_assoc r,
    mgd.mgi_refassoctype t,
    mgd.bib_citation_cache c,
    mgd.mgi_user u1,
    mgd.mgi_user u2,
    mgd.mrk_marker m,
    mgd.acc_accession aa
  WHERE ((r._mgitype_key = 2) AND (r._refassoctype_key = t._refassoctype_key) AND (r._refs_key = c._refs_key) AND (r._createdby_key = u1._user_key) AND (r._modifiedby_key = u2._user_key) AND (r._object_key = m._marker_key) AND (m._marker_key = aa._object_key) AND (aa._mgitype_key = 2) AND (aa._logicaldb_key = 1) AND (aa.preferred = 1))
  ORDER BY m.symbol, t.assoctype;


CREATE VIEW mgd.mgi_relationship_fear_view AS
 SELECT r._relationship_key,
    r._category_key,
    r._object_key_1,
    r._object_key_2,
    r._relationshipterm_key,
    r._qualifier_key,
    r._evidence_key,
    r._refs_key,
    r._createdby_key,
    r._modifiedby_key,
    r.creation_date,
    r.modification_date,
    v1.name AS categoryterm,
    v2.term AS relationshipterm,
    v3.term AS qualifierterm,
    v4.term AS evidenceterm,
    a.symbol AS allelesymbol,
    m.symbol AS markersymbol,
    c.jnumid,
    c.numericpart AS jnum,
    c.short_citation,
    u2.login AS modifiedby,
    u1.login AS createdby,
    aa.accid AS alleleaccid,
    ma.accid AS markeraccid,
    o._organism_key,
    o.commonname AS organism
   FROM mgd.mgi_relationship r,
    mgd.bib_citation_cache c,
    mgd.mgi_user u1,
    mgd.mgi_user u2,
    mgd.mgi_relationship_category v1,
    mgd.voc_term v2,
    mgd.voc_term v3,
    mgd.voc_term v4,
    mgd.all_allele a,
    mgd.mrk_marker m,
    mgd.acc_accession aa,
    mgd.acc_accession ma,
    mgd.mgi_organism o
  WHERE ((r._category_key = ANY (ARRAY[1003, 1004, 1006])) AND (r._category_key = v1._category_key) AND (r._relationshipterm_key = v2._term_key) AND (r._qualifier_key = v3._term_key) AND (r._evidence_key = v4._term_key) AND (r._object_key_1 = a._allele_key) AND (r._object_key_2 = m._marker_key) AND (m._organism_key = 1) AND (m._organism_key = o._organism_key) AND (r._refs_key = c._refs_key) AND (r._createdby_key = u1._user_key) AND (r._modifiedby_key = u2._user_key) AND (r._object_key_1 = aa._object_key) AND (aa._mgitype_key = 11) AND (aa._logicaldb_key = 1) AND (aa.preferred = 1) AND (r._object_key_2 = ma._object_key) AND (ma._mgitype_key = 2) AND (ma._logicaldb_key = 1) AND (ma.preferred = 1))
UNION
 SELECT r._relationship_key,
    r._category_key,
    r._object_key_1,
    r._object_key_2,
    r._relationshipterm_key,
    r._qualifier_key,
    r._evidence_key,
    r._refs_key,
    r._createdby_key,
    r._modifiedby_key,
    r.creation_date,
    r.modification_date,
    v1.name AS categoryterm,
    v2.term AS relationshipterm,
    v3.term AS qualifierterm,
    v4.term AS evidenceterm,
    a.symbol AS allelesymbol,
    m.symbol AS markersymbol,
    c.jnumid,
    c.numericpart AS jnum,
    c.short_citation,
    u2.login AS modifiedby,
    u1.login AS createdby,
    aa.accid AS alleleaccid,
    ma.accid AS markeraccid,
    o._organism_key,
    o.commonname AS organism
   FROM (mgd.mgi_relationship r
     LEFT JOIN mgd.acc_accession ma ON (((r._object_key_2 = ma._object_key) AND (ma._mgitype_key = 2) AND (ma._logicaldb_key = ANY (ARRAY[55, 114])) AND (ma.preferred = 1)))),
    mgd.bib_citation_cache c,
    mgd.mgi_user u1,
    mgd.mgi_user u2,
    mgd.mgi_relationship_category v1,
    mgd.voc_term v2,
    mgd.voc_term v3,
    mgd.voc_term v4,
    mgd.all_allele a,
    mgd.mrk_marker m,
    mgd.acc_accession aa,
    mgd.mgi_organism o
  WHERE ((r._category_key = ANY (ARRAY[1004, 1006])) AND (r._category_key = v1._category_key) AND (r._relationshipterm_key = v2._term_key) AND (r._qualifier_key = v3._term_key) AND (r._evidence_key = v4._term_key) AND (r._object_key_1 = a._allele_key) AND (r._object_key_2 = m._marker_key) AND (m._organism_key <> 1) AND (m._organism_key = o._organism_key) AND (r._refs_key = c._refs_key) AND (r._createdby_key = u1._user_key) AND (r._modifiedby_key = u2._user_key) AND (r._object_key_1 = aa._object_key) AND (aa._mgitype_key = 11) AND (aa._logicaldb_key = 1) AND (aa.preferred = 1));


CREATE VIEW mgd.mgi_relationship_markerqtlcandidate_view AS
 SELECT DISTINCT r._relationship_key,
    r._object_key_1,
    r._object_key_2,
    m1.symbol AS marker1,
    m2.symbol AS marker2,
    c.jnumid
   FROM mgd.mgi_relationship r,
    mgd.mrk_marker m1,
    mgd.mrk_marker m2,
    mgd.bib_citation_cache c
  WHERE ((r._category_key = 1009) AND (r._object_key_1 = m1._marker_key) AND (r._object_key_2 = m2._marker_key) AND (r._refs_key = c._refs_key))
  ORDER BY r._object_key_1, r._object_key_2, c.jnumid;


CREATE VIEW mgd.mgi_relationship_markerqtlinteraction_view AS
 SELECT DISTINCT r._relationship_key,
    r._object_key_1,
    r._object_key_2,
    m1.symbol AS marker1,
    m2.symbol AS marker2
   FROM mgd.mgi_relationship r,
    mgd.mrk_marker m1,
    mgd.mrk_marker m2
  WHERE ((r._category_key = 1010) AND (r._object_key_1 = m1._marker_key) AND (r._object_key_2 = m2._marker_key));


CREATE VIEW mgd.mgi_relationship_markertss_view AS
 SELECT r._relationship_key,
    r._category_key,
    r._object_key_1,
    r._object_key_2,
    r._relationshipterm_key,
    r._qualifier_key,
    r._evidence_key,
    r._refs_key,
    r._createdby_key,
    r._modifiedby_key,
    r.creation_date,
    r.modification_date,
    v1.name AS categoryterm,
    v2.term AS relationshipterm,
    v3.term AS qualifierterm,
    v4.term AS evidenceterm,
    m1.symbol AS marker1,
    m2.symbol AS marker2,
    c.jnumid,
    c.numericpart AS jnum,
    c.short_citation,
    u2.login AS modifiedby,
    u1.login AS createdby
   FROM mgd.mgi_relationship r,
    mgd.bib_citation_cache c,
    mgd.mgi_user u1,
    mgd.mgi_user u2,
    mgd.mgi_relationship_category v1,
    mgd.voc_term v2,
    mgd.voc_term v3,
    mgd.voc_term v4,
    mgd.mrk_marker m1,
    mgd.mrk_marker m2
  WHERE ((r._category_key = 1008) AND (r._category_key = v1._category_key) AND (r._relationshipterm_key = v2._term_key) AND (r._qualifier_key = v3._term_key) AND (r._evidence_key = v4._term_key) AND (r._object_key_1 = m1._marker_key) AND (r._object_key_2 = m2._marker_key) AND (r._refs_key = c._refs_key) AND (r._createdby_key = u1._user_key) AND (r._modifiedby_key = u2._user_key));


CREATE VIEW mgd.mgi_synonym_allele_view AS
 SELECT s._synonym_key,
    s._object_key,
    s._mgitype_key,
    s._synonymtype_key,
    s._refs_key,
    s.synonym,
    s._createdby_key,
    s._modifiedby_key,
    s.creation_date,
    s.modification_date,
    t._organism_key,
    t.synonymtype,
    t.allowonlyone,
    c.jnumid,
    c.numericpart AS jnum,
    c.short_citation,
    u1.login AS modifiedby
   FROM mgd.mgi_synonym s,
    mgd.mgi_synonymtype t,
    mgd.bib_citation_cache c,
    mgd.mgi_user u1
  WHERE ((s._synonymtype_key = t._synonymtype_key) AND (t._mgitype_key = 11) AND (s._refs_key = c._refs_key) AND (s._modifiedby_key = u1._user_key));


CREATE VIEW mgd.mgi_synonym_musmarker_view AS
 SELECT s._synonym_key,
    s._object_key,
    s._mgitype_key,
    s._synonymtype_key,
    s._refs_key,
    s.synonym,
    s._createdby_key,
    s._modifiedby_key,
    s.creation_date,
    s.modification_date,
    t._organism_key,
    t.synonymtype,
    t.allowonlyone,
    c.jnumid,
    c.numericpart AS jnum,
    c.short_citation,
    u2.login AS modifiedby,
    u1.login AS createdby
   FROM (mgd.mgi_synonym s
     LEFT JOIN mgd.bib_citation_cache c ON ((s._refs_key = c._refs_key))),
    mgd.mgi_synonymtype t,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((s._synonymtype_key = t._synonymtype_key) AND (t._mgitype_key = 2) AND (t._organism_key = 1) AND (s._createdby_key = u1._user_key) AND (s._modifiedby_key = u2._user_key));


CREATE VIEW mgd.mgi_synonymtype_strain_view AS
 SELECT mgi_synonymtype._synonymtype_key,
    mgi_synonymtype._mgitype_key,
    mgi_synonymtype._organism_key,
    mgi_synonymtype.synonymtype,
    mgi_synonymtype.definition,
    mgi_synonymtype.allowonlyone,
    mgi_synonymtype._createdby_key,
    mgi_synonymtype._modifiedby_key,
    mgi_synonymtype.creation_date,
    mgi_synonymtype.modification_date
   FROM mgd.mgi_synonymtype
  WHERE (mgi_synonymtype._mgitype_key = 10);


CREATE VIEW mgd.mgi_user_active_view AS
 SELECT u._user_key,
    u._usertype_key,
    u._userstatus_key,
    u.login,
    u.name,
    u.orcid,
    u._group_key,
    u._createdby_key,
    u._modifiedby_key,
    u.creation_date,
    u.modification_date,
    t2.term AS usertype,
    u1.login AS createdby,
    u2.login AS modifiedby
   FROM mgd.mgi_user u,
    mgd.voc_term t1,
    mgd.voc_vocab v1,
    mgd.voc_term t2,
    mgd.voc_vocab v2,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((u._userstatus_key = t1._term_key) AND (t1.term = 'Active'::text) AND (t1._vocab_key = v1._vocab_key) AND (v1.name = 'User Status'::text) AND (u._usertype_key = t2._term_key) AND (t2._vocab_key = v2._vocab_key) AND (v2.name = 'User Type'::text) AND (u._createdby_key = u1._user_key) AND (u._modifiedby_key = u2._user_key));


CREATE VIEW mgd.mld_expt_marker_view AS
 SELECT DISTINCT c.jnumid,
    c.jnum,
    c.short_citation,
    m.symbol,
    x.expttype,
    x.tag,
    e._assoc_key,
    e._expt_key,
    e._marker_key,
    e._allele_key,
    e._assay_type_key,
    e.sequencenum,
    e.description,
    e.matrixdata,
    e.creation_date,
    e.modification_date,
    al.symbol AS allele,
    a.description AS assay,
    c._primary,
    c.authors,
    m.chromosome,
    aa.accid
   FROM ((((((mgd.bib_view c
     JOIN mgd.mld_expts x ON ((c._refs_key = x._refs_key)))
     JOIN mgd.mld_expt_marker e ON ((x._expt_key = e._expt_key)))
     JOIN mgd.mld_assay_types a ON ((e._assay_type_key = a._assay_type_key)))
     LEFT JOIN mgd.all_allele al ON ((e._allele_key = al._allele_key)))
     LEFT JOIN mgd.mrk_marker m ON ((e._marker_key = m._marker_key)))
     JOIN mgd.acc_accession aa ON (((m._marker_key = aa._object_key) AND (aa._mgitype_key = 2) AND (aa._logicaldb_key = 1) AND (aa.preferred = 1))));


CREATE VIEW mgd.mld_expt_view AS
 SELECT c.jnumid,
    c.jnum,
    c.short_citation,
    x._expt_key,
    x._refs_key,
    x.expttype,
    x.tag,
    x.chromosome,
    x.creation_date,
    x.modification_date,
    c._primary,
    c.authors,
    a.accid AS mgiid,
    a.prefixpart,
    a.numericpart,
    ((((x.expttype || '-'::text) || ((x.tag)::character varying(5))::text) || ', Chr '::text) || x.chromosome) AS exptlabel
   FROM mgd.bib_view c,
    mgd.mld_expts x,
    mgd.acc_accession a
  WHERE ((c._refs_key = x._refs_key) AND (x._expt_key = a._object_key) AND (a._mgitype_key = 4) AND (a._logicaldb_key = 1) AND (a.prefixpart = 'MGI:'::text) AND (a.preferred = 1));


CREATE VIEW mgd.mrk_acc_view AS
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    l.name AS logicaldb,
    m._organism_key
   FROM mgd.acc_accession a,
    mgd.acc_logicaldb l,
    mgd.mrk_marker m
  WHERE ((a._mgitype_key = 2) AND (a._logicaldb_key = l._logicaldb_key) AND (a._object_key = m._marker_key));


CREATE VIEW mgd.mrk_accnoref_view AS
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    l.name AS logicaldb,
    m.name AS mgitype,
    mt.name AS subtype,
    ((((ma.symbol || ', '::text) || ma.name) || ', Chr '::text) || ma.chromosome) AS description
   FROM mgd.acc_accession a,
    mgd.acc_logicaldb l,
    mgd.acc_mgitype m,
    mgd.mrk_marker ma,
    mgd.mrk_types mt
  WHERE ((a._mgitype_key = 2) AND (a._logicaldb_key = l._logicaldb_key) AND (a._mgitype_key = m._mgitype_key) AND (a._object_key = ma._marker_key) AND (ma._marker_type_key = mt._marker_type_key) AND (NOT (EXISTS ( SELECT r._accession_key,
            r._refs_key,
            r._createdby_key,
            r._modifiedby_key,
            r.creation_date,
            r.modification_date
           FROM mgd.acc_accessionreference r
          WHERE (a._accession_key = r._accession_key)))) AND (a._logicaldb_key <> ALL (ARRAY[117, 118])));


CREATE VIEW mgd.mrk_accref1_view AS
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    l.name AS logicaldb,
    m.name AS mgitype,
    ar._refs_key,
    b.numericpart AS jnum,
    b.short_citation,
    b.jnumid,
    u2.login AS modifiedby,
    u1.login AS createdby
   FROM mgd.acc_accession a,
    mgd.acc_logicaldb l,
    mgd.acc_mgitype m,
    mgd.acc_accessionreference ar,
    mgd.bib_citation_cache b,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((a._mgitype_key = 2) AND (a._accession_key = ar._accession_key) AND (a._logicaldb_key = l._logicaldb_key) AND (a._mgitype_key = m._mgitype_key) AND (ar._refs_key = b._refs_key) AND (a._createdby_key = u1._user_key) AND (a._modifiedby_key = u2._user_key) AND (a._logicaldb_key = ANY (ARRAY[8, 9])));


CREATE VIEW mgd.mrk_accref2_view AS
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    l.name AS logicaldb,
    m.name AS mgitype,
    ar._refs_key,
    b.numericpart AS jnum,
    b.short_citation,
    b.jnumid,
    u2.login AS modifiedby,
    u1.login AS createdby
   FROM mgd.acc_accession a,
    mgd.acc_logicaldb l,
    mgd.acc_mgitype m,
    mgd.acc_accessionreference ar,
    mgd.bib_citation_cache b,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((a._mgitype_key = 2) AND (a._accession_key = ar._accession_key) AND (a._logicaldb_key = l._logicaldb_key) AND (a._mgitype_key = m._mgitype_key) AND (ar._refs_key = b._refs_key) AND (a._createdby_key = u1._user_key) AND (a._modifiedby_key = u2._user_key) AND (a._logicaldb_key <> ALL (ARRAY[1, 8, 9, 117, 118])));


CREATE VIEW mgd.mrk_current_view AS
 SELECT c._current_key,
    c._marker_key,
    c.creation_date,
    c.modification_date,
    m1.symbol AS current_symbol,
    m2.symbol,
    m1.chromosome,
    m1._marker_type_key
   FROM mgd.mrk_current c,
    mgd.mrk_marker m1,
    mgd.mrk_marker m2
  WHERE ((c._marker_key = m2._marker_key) AND (c._current_key = m1._marker_key));


CREATE VIEW mgd.mrk_history_view AS
 SELECT h._assoc_key,
    h._marker_key,
    h._marker_event_key,
    h._marker_eventreason_key,
    h._history_key,
    h._refs_key,
    h.sequencenum,
    h.name,
    h.event_date,
    h._createdby_key,
    h._modifiedby_key,
    h.creation_date,
    h.modification_date,
    (h.event_date)::character(10) AS event_display,
    e.term AS event,
    er.term AS eventreason,
    m1.symbol AS history,
    m1.name AS historyname,
    m2.symbol,
    m2.name AS markername,
    u1.login AS createdby,
    u2.login AS modifiedby,
    c.short_citation,
    c.jnumid
   FROM (mgd.mrk_history h
     LEFT JOIN mgd.bib_citation_cache c ON ((h._refs_key = c._refs_key))),
    mgd.voc_term e,
    mgd.voc_term er,
    mgd.mrk_marker m1,
    mgd.mrk_marker m2,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((h._marker_event_key = e._term_key) AND (h._marker_eventreason_key = er._term_key) AND (h._marker_key = m2._marker_key) AND (h._history_key = m1._marker_key) AND (h._createdby_key = u1._user_key) AND (h._modifiedby_key = u2._user_key));


CREATE VIEW mgd.mrk_marker_view AS
 SELECT m._marker_key,
    m._organism_key,
    m._marker_status_key,
    m._marker_type_key,
    m.symbol,
    m.name,
    m.chromosome,
    m.cytogeneticoffset,
    m.cmoffset,
    m._createdby_key,
    m._modifiedby_key,
    m.creation_date,
    m.modification_date,
    (((s.commonname || ' ('::text) || s.latinname) || ')'::text) AS organism,
    s.commonname,
    s.latinname,
    ms.status,
    mt.name AS markertype,
    u1.login AS createdby,
    u2.login AS modifiedby
   FROM mgd.mrk_marker m,
    mgd.mgi_organism s,
    mgd.mrk_status ms,
    mgd.mrk_types mt,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((m._organism_key = s._organism_key) AND (m._marker_status_key = ms._marker_status_key) AND (m._marker_type_key = mt._marker_type_key) AND (m._createdby_key = u1._user_key) AND (m._modifiedby_key = u2._user_key));


CREATE VIEW mgd.mrk_mouse_view AS
 SELECT m._marker_key,
    m._organism_key,
    m._marker_status_key,
    m._marker_type_key,
    m.symbol,
    m.name,
    m.chromosome,
    m.cytogeneticoffset,
    m.cmoffset,
    m._createdby_key,
    m._modifiedby_key,
    m.creation_date,
    m.modification_date,
    (((s.commonname || ' ('::text) || s.latinname) || ')'::text) AS organism,
    s.commonname,
    s.latinname,
    ms.status,
    a.accid AS mgiid,
    a.prefixpart,
    a.numericpart,
    a._accession_key,
    t.name AS markertype
   FROM mgd.mrk_marker m,
    mgd.mgi_organism s,
    mgd.mrk_status ms,
    mgd.acc_accession a,
    mgd.mrk_types t
  WHERE ((m._organism_key = 1) AND (m._organism_key = s._organism_key) AND (m._marker_status_key = ms._marker_status_key) AND (m._marker_key = a._object_key) AND (a._mgitype_key = 2) AND (a._logicaldb_key = 1) AND (a.prefixpart = 'MGI:'::text) AND (a.preferred = 1) AND (m._marker_type_key = t._marker_type_key));


CREATE VIEW mgd.mrk_summary_view AS
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    a2.accid AS mgiid,
    mt.name AS subtype,
    ((((m.symbol || ', '::text) || m.name) || ', Chr '::text) || m.chromosome) AS description,
    m.symbol AS short_description
   FROM mgd.acc_accession a,
    mgd.acc_accession a2,
    mgd.mrk_marker m,
    mgd.mrk_types mt
  WHERE ((m._organism_key = 1) AND (m._marker_type_key = mt._marker_type_key) AND (m._marker_key = a._object_key) AND (a._mgitype_key = 2) AND (a.private = 0) AND (a._object_key = a2._object_key) AND (a2._logicaldb_key = 1) AND (a2._mgitype_key = 2) AND (a2.prefixpart = 'MGI:'::text) AND (a2.preferred = 1));


CREATE VIEW mgd.mrk_summarybyreference_view AS
 SELECT DISTINCT aa.jnumid,
    m._marker_key,
    m.symbol,
    m.name,
    a.accid,
    t1.status AS markerstatus,
    t2.name AS markertype,
    array_to_string(array_agg(DISTINCT vt.term), ','::text) AS featuretypes,
    array_to_string(array_agg(DISTINCT s.synonym), ','::text) AS synonyms
   FROM mgd.bib_citation_cache aa,
    mgd.mrk_reference r,
    (mgd.mrk_marker m
     LEFT JOIN mgd.mgi_synonym s ON (((m._marker_key = s._object_key) AND (s._mgitype_key = 2)))),
    mgd.acc_accession a,
    mgd.mrk_status t1,
    mgd.mrk_types t2,
    mgd.voc_annot va,
    mgd.voc_term vt
  WHERE ((aa._refs_key = r._refs_key) AND (r._marker_key = m._marker_key) AND (m._marker_key = a._object_key) AND (a._mgitype_key = 2) AND (a._logicaldb_key = 1) AND (a.preferred = 1) AND (m._marker_status_key = t1._marker_status_key) AND (m._marker_type_key = t2._marker_type_key) AND (m._marker_key = va._object_key) AND (va._annottype_key = 1011) AND (va._term_key = vt._term_key))
  GROUP BY aa.jnumid, m._marker_key, m.symbol, m.name, a.accid, t1.status, t2.name, vt.term
UNION
 SELECT DISTINCT aa.jnumid,
    m._marker_key,
    m.symbol,
    m.name,
    a.accid,
    t1.status AS markerstatus,
    t2.name AS markertype,
    NULL::text AS featuretypes,
    array_to_string(array_agg(DISTINCT s.synonym), ','::text) AS synonyms
   FROM mgd.bib_citation_cache aa,
    mgd.mrk_reference r,
    (mgd.mrk_marker m
     LEFT JOIN mgd.mgi_synonym s ON (((m._marker_key = s._object_key) AND (s._mgitype_key = 2)))),
    mgd.acc_accession a,
    mgd.mrk_status t1,
    mgd.mrk_types t2
  WHERE ((aa._refs_key = r._refs_key) AND (r._marker_key = m._marker_key) AND (m._marker_key = a._object_key) AND (a._mgitype_key = 2) AND (a._logicaldb_key = 1) AND (a.preferred = 1) AND (m._marker_status_key = t1._marker_status_key) AND (m._marker_type_key = t2._marker_type_key) AND (NOT (EXISTS ( SELECT 1
           FROM mgd.voc_annot va
          WHERE ((m._marker_key = va._object_key) AND (va._annottype_key = 1011))))))
  GROUP BY aa.jnumid, m._marker_key, m.symbol, m.name, a.accid, t1.status, t2.name
  ORDER BY 6, 3;


CREATE VIEW mgd.prb_acc_view AS
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    l.name AS logicaldb,
    p.name AS description
   FROM mgd.acc_accession a,
    mgd.acc_logicaldb l,
    mgd.prb_probe p
  WHERE ((a._mgitype_key = 3) AND (a._logicaldb_key = l._logicaldb_key) AND (a._object_key = p._probe_key));


CREATE VIEW mgd.prb_accref_view AS
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    l.name AS logicaldb,
    m.name AS mgitype,
    r._reference_key
   FROM mgd.acc_accession a,
    mgd.acc_logicaldb l,
    mgd.acc_mgitype m,
    mgd.acc_accessionreference ar,
    mgd.prb_reference r
  WHERE ((a._mgitype_key = 3) AND (a._accession_key = ar._accession_key) AND (a._logicaldb_key = l._logicaldb_key) AND (a._mgitype_key = m._mgitype_key) AND (a._object_key = r._probe_key) AND (ar._refs_key = r._refs_key));


CREATE VIEW mgd.prb_marker_view AS
 SELECT g._probe_key,
    p.name,
    g._marker_key,
    m.symbol,
    m.chromosome,
    g.relationship,
    g._refs_key,
    b.numericpart AS jnum,
    b.short_citation,
    u.login AS modifiedby,
    g.modification_date,
    g._modifiedby_key
   FROM mgd.prb_probe p,
    mgd.prb_marker g,
    mgd.mrk_marker m,
    mgd.bib_citation_cache b,
    mgd.mgi_user u
  WHERE ((p._probe_key = g._probe_key) AND (g._marker_key = m._marker_key) AND (g._refs_key = b._refs_key) AND (g._modifiedby_key = u._user_key));


CREATE VIEW mgd.prb_probe_view AS
 SELECT p._probe_key,
    p.name,
    p.derivedfrom,
    p.ampprimer,
    p._source_key,
    p._vector_key,
    p._segmenttype_key,
    p.primer1sequence,
    p.primer2sequence,
    p.regioncovered,
    p.insertsite,
    p.insertsize,
    p.productsize,
    p._createdby_key,
    p._modifiedby_key,
    p.creation_date,
    p.modification_date,
    v1.term AS segmenttype,
    v2.term AS vectortype,
    p2.name AS parentclone,
    u1.login AS createdby,
    u2.login AS modifiedby,
    a.accid AS mgiid,
    a.prefixpart,
    a.numericpart
   FROM ((((((mgd.prb_probe p
     JOIN mgd.acc_accession a ON (((p._probe_key = a._object_key) AND (a._mgitype_key = 3) AND (a._logicaldb_key = 1) AND (a.prefixpart = 'MGI:'::text) AND (a.preferred = 1))))
     JOIN mgd.voc_term v1 ON ((p._segmenttype_key = v1._term_key)))
     JOIN mgd.voc_term v2 ON ((p._vector_key = v2._term_key)))
     JOIN mgd.mgi_user u1 ON ((p._createdby_key = u1._user_key)))
     JOIN mgd.mgi_user u2 ON ((p._modifiedby_key = u2._user_key)))
     LEFT JOIN mgd.prb_probe p2 ON ((p.derivedfrom = p2._probe_key)));


CREATE VIEW mgd.prb_source_acc_view AS
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    l.name AS logicaldb
   FROM mgd.acc_accession a,
    mgd.acc_logicaldb l
  WHERE ((a._mgitype_key = 5) AND (a._logicaldb_key = l._logicaldb_key));


CREATE VIEW mgd.prb_source_view AS
 SELECT p._source_key,
    p._segmenttype_key,
    p._vector_key,
    p._organism_key,
    p._strain_key,
    p._tissue_key,
    p._gender_key,
    p._cellline_key,
    p._refs_key,
    p.name,
    p.description,
    p.age,
    p.agemin,
    p.agemax,
    p.iscuratoredited,
    p._createdby_key,
    p._modifiedby_key,
    p.creation_date,
    p.modification_date,
    c.commonname AS organism,
    s.strain,
    s.standard AS sstandard,
    t.tissue,
    t.standard AS tstandard,
    v1.term AS gender,
    v2.term AS cellline,
    v3.term AS segmenttype,
    v4.term AS vectortype,
    u1.login AS createdby,
    u2.login AS modifiedby
   FROM mgd.prb_source p,
    mgd.mgi_organism c,
    mgd.prb_strain s,
    mgd.prb_tissue t,
    mgd.voc_term v1,
    mgd.voc_term v2,
    mgd.voc_term v3,
    mgd.voc_term v4,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((p._organism_key = c._organism_key) AND (p._strain_key = s._strain_key) AND (p._tissue_key = t._tissue_key) AND (p._gender_key = v1._term_key) AND (p._cellline_key = v2._term_key) AND (p._segmenttype_key = v3._term_key) AND (p._vector_key = v4._term_key) AND (p._createdby_key = u1._user_key) AND (p._modifiedby_key = u2._user_key));


CREATE VIEW mgd.prb_strain_acc_view AS
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    l.name AS logicaldb
   FROM mgd.acc_accession a,
    mgd.acc_logicaldb l
  WHERE ((a._mgitype_key = 10) AND (a._logicaldb_key = l._logicaldb_key));


CREATE VIEW mgd.prb_strain_attribute_view AS
 SELECT va._annot_key,
    va._object_key AS _strain_key,
    va._term_key,
    vt.term
   FROM mgd.voc_annot va,
    mgd.voc_annottype vat,
    mgd.voc_term vt
  WHERE ((vat.name = 'Strain/Attributes'::text) AND (vat._annottype_key = va._annottype_key) AND (va._term_key = vt._term_key));


CREATE VIEW mgd.prb_strain_genotype_view AS
 SELECT s._straingenotype_key,
    s._strain_key,
    s._genotype_key,
    s._qualifier_key,
    s._createdby_key,
    s._modifiedby_key,
    s.creation_date,
    s.modification_date,
    ss.strain,
    gs.strain AS description,
    a.accid AS mgiid,
    t.term AS qualifier,
    u.login AS modifiedby
   FROM mgd.prb_strain_genotype s,
    mgd.prb_strain ss,
    mgd.acc_accession a,
    mgd.gxd_genotype g,
    mgd.prb_strain gs,
    mgd.voc_term t,
    mgd.mgi_user u
  WHERE ((s._strain_key = ss._strain_key) AND (s._qualifier_key = t._term_key) AND (s._genotype_key = a._object_key) AND (a._mgitype_key = 12) AND (a._logicaldb_key = 1) AND (s._genotype_key = g._genotype_key) AND (g._strain_key = gs._strain_key) AND (s._modifiedby_key = u._user_key));


CREATE VIEW mgd.prb_strain_marker_view AS
 SELECT s._strainmarker_key,
    s._strain_key,
    s._marker_key,
    s._allele_key,
    s._qualifier_key,
    s._createdby_key,
    s._modifiedby_key,
    s.creation_date,
    s.modification_date,
    m.symbol,
    m.chromosome,
    c.sequencenum,
    a.symbol AS allelesymbol,
    t.term AS qualifier
   FROM ((((mgd.prb_strain_marker s
     LEFT JOIN mgd.mrk_marker m ON ((s._marker_key = m._marker_key)))
     LEFT JOIN mgd.mrk_chromosome c ON (((m._organism_key = c._organism_key) AND (m.chromosome = c.chromosome))))
     LEFT JOIN mgd.all_allele a ON ((s._allele_key = a._allele_key)))
     JOIN mgd.voc_term t ON ((s._qualifier_key = t._term_key)));


CREATE VIEW mgd.prb_strain_needsreview_view AS
 SELECT va._annot_key,
    va._annottype_key,
    va._object_key,
    va._term_key,
    va._qualifier_key,
    va.creation_date,
    va.modification_date,
    vt.term
   FROM mgd.voc_annot va,
    mgd.voc_term vt
  WHERE ((va._annottype_key = 1008) AND (va._term_key = vt._term_key));


CREATE VIEW mgd.prb_strain_reference_view AS
 SELECT DISTINCT m._strain_key,
    e._refs_key,
    'Mapping'::text AS dataset
   FROM mgd.mld_expts e,
    mgd.mld_insitu m
  WHERE (e._expt_key = m._expt_key)
UNION
 SELECT DISTINCT m._strain_key,
    e._refs_key,
    'Mapping'::text AS dataset
   FROM mgd.mld_expts e,
    mgd.mld_fish m
  WHERE (e._expt_key = m._expt_key)
UNION
 SELECT DISTINCT c._femalestrain_key AS _strain_key,
    e._refs_key,
    'Mapping'::text AS dataset
   FROM mgd.mld_expts e,
    mgd.mld_matrix m,
    mgd.crs_cross c
  WHERE ((e._expt_key = m._expt_key) AND (m._cross_key = c._cross_key))
UNION
 SELECT DISTINCT c._malestrain_key AS _strain_key,
    e._refs_key,
    'Mapping'::text AS dataset
   FROM mgd.mld_expts e,
    mgd.mld_matrix m,
    mgd.crs_cross c
  WHERE ((e._expt_key = m._expt_key) AND (m._cross_key = c._cross_key))
UNION
 SELECT DISTINCT c._strainho_key AS _strain_key,
    e._refs_key,
    'Mapping'::text AS dataset
   FROM mgd.mld_expts e,
    mgd.mld_matrix m,
    mgd.crs_cross c
  WHERE ((e._expt_key = m._expt_key) AND (m._cross_key = c._cross_key))
UNION
 SELECT DISTINCT c._strainht_key AS _strain_key,
    e._refs_key,
    'Mapping'::text AS dataset
   FROM mgd.mld_expts e,
    mgd.mld_matrix m,
    mgd.crs_cross c
  WHERE ((e._expt_key = m._expt_key) AND (m._cross_key = c._cross_key))
UNION
 SELECT DISTINCT s._strain_key,
    x._refs_key,
    'Expression'::text AS dataset
   FROM mgd.gxd_genotype s,
    mgd.gxd_expression x
  WHERE (s._genotype_key = x._genotype_key)
UNION
 SELECT DISTINCT s._strain_key,
    r._refs_key,
    'RFLP'::text AS dataset
   FROM mgd.prb_reference r,
    mgd.prb_rflv v,
    mgd.prb_allele a,
    mgd.prb_allele_strain s
  WHERE ((r._reference_key = v._reference_key) AND (v._rflv_key = a._rflv_key) AND (a._allele_key = s._allele_key))
UNION
 SELECT DISTINCT a._strain_key,
    r._refs_key,
    'Allele'::text AS dataset
   FROM mgd.all_allele a,
    mgd.mgi_reference_assoc r
  WHERE ((a._allele_key = r._object_key) AND (r._mgitype_key = 11));


CREATE VIEW mgd.prb_strain_view AS
 SELECT s._strain_key,
    s._species_key,
    s._straintype_key,
    s.strain,
    s.standard,
    s.private,
    s.geneticbackground,
    s._createdby_key,
    s._modifiedby_key,
    s.creation_date,
    s.modification_date,
    sp.term AS species,
    st.term AS straintype,
    u1.login AS createdby,
    u2.login AS modifiedby
   FROM mgd.prb_strain s,
    mgd.voc_term sp,
    mgd.voc_term st,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((s._species_key = sp._term_key) AND (s._straintype_key = st._term_key) AND (s._createdby_key = u1._user_key) AND (s._modifiedby_key = u2._user_key));


CREATE VIEW mgd.seq_sequence_acc_view AS
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    l.name AS logicaldb
   FROM mgd.acc_accession a,
    mgd.acc_logicaldb l
  WHERE ((a._mgitype_key = 19) AND (a._logicaldb_key = l._logicaldb_key));


CREATE VIEW mgd.seq_summary_view AS
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    a.accid AS mgiid,
    v1.term AS subtype,
    s.description,
    s.description AS short_description
   FROM mgd.acc_accession a,
    mgd.seq_sequence s,
    mgd.voc_term v1
  WHERE ((a._mgitype_key = 19) AND (a.private = 0) AND (a._object_key = s._sequence_key) AND (s._sequencetype_key = v1._term_key));


CREATE VIEW mgd.voc_term_view AS
 SELECT t._term_key,
    t._vocab_key,
    t.term,
    t.abbreviation,
    t.note,
    t.sequencenum,
    t.isobsolete,
    t._createdby_key,
    t._modifiedby_key,
    t.creation_date,
    t.modification_date,
    v.name AS vocabname,
    a.accid,
    a._logicaldb_key,
        CASE
            WHEN (t.isobsolete = 1) THEN 'Yes'::text
            WHEN (t.isobsolete = 0) THEN 'No'::text
            ELSE NULL::text
        END AS obsoletestate
   FROM ((mgd.voc_term t
     JOIN mgd.voc_vocab v ON ((t._vocab_key = v._vocab_key)))
     LEFT JOIN mgd.acc_accession a ON (((t._term_key = a._object_key) AND (a._mgitype_key = 13) AND (a.preferred = 1))));


CREATE VIEW mgd.voc_annot_view AS
 SELECT v._annot_key,
    v._annottype_key,
    v._object_key,
    v._term_key,
    v._qualifier_key,
    v.creation_date,
    v.modification_date,
    q.abbreviation AS qualifier,
    t.term,
    t.sequencenum,
    t.accid,
    t._logicaldb_key,
    a._vocab_key,
    a._mgitype_key,
    a._evidencevocab_key,
    a.name AS annottype
   FROM mgd.voc_annot v,
    mgd.voc_term_view t,
    mgd.voc_annottype a,
    mgd.voc_term q
  WHERE ((v._term_key = t._term_key) AND (v._annottype_key = a._annottype_key) AND (v._qualifier_key = q._term_key));


CREATE VIEW mgd.voc_evidence_view AS
 SELECT e._annotevidence_key,
    e._annot_key,
    e._evidenceterm_key,
    e._refs_key,
    e.inferredfrom,
    e._createdby_key,
    e._modifiedby_key,
    e.creation_date,
    e.modification_date,
    t.abbreviation AS evidencecode,
    t.sequencenum AS evidenceseqnum,
    c.jnumid,
    c.numericpart AS jnum,
    c.short_citation,
    u1.login AS createdby,
    u2.login AS modifiedby
   FROM mgd.voc_evidence e,
    mgd.voc_term t,
    mgd.bib_citation_cache c,
    mgd.mgi_user u1,
    mgd.mgi_user u2
  WHERE ((e._evidenceterm_key = t._term_key) AND (e._refs_key = c._refs_key) AND (e._createdby_key = u1._user_key) AND (e._modifiedby_key = u2._user_key));


CREATE VIEW mgd.voc_term_acc_view AS
 SELECT a._accession_key,
    a.accid,
    a.prefixpart,
    a.numericpart,
    a._logicaldb_key,
    a._object_key,
    a._mgitype_key,
    a.private,
    a.preferred,
    a._createdby_key,
    a._modifiedby_key,
    a.creation_date,
    a.modification_date,
    l.name AS logicaldb
   FROM mgd.acc_accession a,
    mgd.acc_logicaldb l
  WHERE ((a._mgitype_key = 13) AND (a._logicaldb_key = l._logicaldb_key));


CREATE VIEW mgd.voc_term_repqualifier_view AS
 SELECT v.name,
    t._term_key,
    t._vocab_key,
    t.term,
    t.abbreviation,
    t.note,
    t.sequencenum,
    t.isobsolete,
    t._createdby_key,
    t._modifiedby_key,
    t.creation_date,
    t.modification_date
   FROM mgd.voc_vocab v,
    mgd.voc_term t
  WHERE ((v.name = 'Representative Sequence Qualifier'::text) AND (v._vocab_key = t._vocab_key));


CREATE VIEW mgd.voc_termfamily_view AS
 SELECT a.accid AS searchid,
    aa.accid,
    anc._term_key,
    anc._vocab_key,
    anc.term,
    anc.abbreviation,
    anc.note,
    anc.sequencenum,
    anc.isobsolete,
    anc._createdby_key,
    anc._modifiedby_key,
    anc.creation_date,
    anc.modification_date
   FROM mgd.voc_term anc,
    mgd.dag_closure dc,
    mgd.acc_accession a,
    mgd.acc_accession aa
  WHERE ((dc._descendentobject_key = a._object_key) AND (a._mgitype_key = 13) AND (dc._ancestorobject_key = anc._term_key) AND (anc._term_key = aa._object_key) AND (aa._mgitype_key = 13) AND (aa.preferred = 1))
UNION
 SELECT a.accid AS searchid,
    aa.accid,
    sib._term_key,
    sib._vocab_key,
    sib.term,
    sib.abbreviation,
    sib.note,
    sib.sequencenum,
    sib.isobsolete,
    sib._createdby_key,
    sib._modifiedby_key,
    sib.creation_date,
    sib.modification_date
   FROM mgd.voc_term sib,
    mgd.dag_edge e1,
    mgd.dag_node e1c,
    mgd.dag_edge e2,
    mgd.dag_node e2c,
    mgd.acc_accession a,
    mgd.acc_accession aa
  WHERE ((e1._child_key = e1c._node_key) AND (e1c._object_key = sib._term_key) AND (e2._child_key = e2c._node_key) AND (e2c._object_key = a._object_key) AND (a._mgitype_key = 13) AND (e1._parent_key = e2._parent_key) AND (sib._term_key = aa._object_key) AND (aa._mgitype_key = 13) AND (aa.preferred = 1))
UNION
 SELECT a.accid AS searchid,
    aa.accid,
    kid._term_key,
    kid._vocab_key,
    kid.term,
    kid.abbreviation,
    kid.note,
    kid.sequencenum,
    kid.isobsolete,
    kid._createdby_key,
    kid._modifiedby_key,
    kid.creation_date,
    kid.modification_date
   FROM mgd.voc_term kid,
    mgd.dag_edge e,
    mgd.dag_node nc,
    mgd.dag_node np,
    mgd.acc_accession a,
    mgd.acc_accession aa
  WHERE ((e._child_key = nc._node_key) AND (nc._object_key = kid._term_key) AND (e._parent_key = np._node_key) AND (np._object_key = a._object_key) AND (a._mgitype_key = 13) AND (kid._term_key = aa._object_key) AND (aa._mgitype_key = 13) AND (aa.preferred = 1))
UNION
 SELECT a.accid AS searchid,
    a.accid,
    t._term_key,
    t._vocab_key,
    t.term,
    t.abbreviation,
    t.note,
    t.sequencenum,
    t.isobsolete,
    t._createdby_key,
    t._modifiedby_key,
    t.creation_date,
    t.modification_date
   FROM mgd.acc_accession a,
    mgd.voc_term t
  WHERE ((a._mgitype_key = 13) AND (a._object_key = t._term_key));


CREATE VIEW mgd.voc_termfamilyedges_view AS
 SELECT e._edge_key,
    tf1.searchid,
    ec._object_key AS _child_key,
    ep._object_key AS _parent_key,
    dl.label
   FROM mgd.dag_edge e,
    mgd.dag_node ec,
    mgd.dag_node ep,
    mgd.dag_label dl,
    mgd.voc_termfamily_view tf1,
    mgd.voc_termfamily_view tf2
  WHERE ((e._child_key = ec._node_key) AND (e._parent_key = ep._node_key) AND (e._label_key = dl._label_key) AND (ec._object_key = tf1._term_key) AND (ep._object_key = tf2._term_key) AND (tf1.searchid = tf2.searchid))
  ORDER BY ec._object_key, ep._object_key;



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


CREATE FUNCTION mgd.acc_accession_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: ACC_Accession_delete()
-- DESCRIPTOIN:
--	
-- 1) If deleting MGI Image Pixel number, then nullify X/Y Dimensions of IMG_Image record TR#134
-- INPUT:
--	none
-- RETURNS:
--	NEW
IF OLD._LogicalDB_key = 19
THEN
        UPDATE IMG_Image
        SET xDim = null, 
            yDim = null
        WHERE OLD._Object_key = IMG_Image._Image_key
	;
        UPDATE IMG_ImagePane
        SET x = null, 
            y = null,
            width = null,
            height = null
        WHERE OLD._Object_key = IMG_ImagePane._Image_key
	;
END IF;
-- if trying to delete a J:
IF OLD._MGIType_key = 1 AND OLD._LogicalDB_key = 1 AND OLD.prefixPart = 'J:'
THEN
        --04/28 : lec & jfinger discussed
        --why would we not want this to happend?
	--IF EXISTS (SELECT * FROM BIB_Refs b, BIB_Workflow_Relevance wr
        --  	WHERE OLD._Object_key = b._Refs_key 
        --        AND b._Refs_key = wr._Refs_key
        --        AND wr.isCurrent = 1
        --        AND wr._Relevance_key = 70594666)
	--THEN
        --	RAISE EXCEPTION E'Cannot delete J:; relevance status = discard';
	--END IF;
	IF EXISTS (SELECT * FROM ALL_Allele
          	WHERE OLD._Object_key = _Refs_key)
	THEN
        	RAISE EXCEPTION E'Cannot delete J:; it is referenced in ALL_Allele(s)';
	END IF;
	IF EXISTS (SELECT * FROM ALL_CellLine_Derivation
          	WHERE OLD._Object_key = _Refs_key)
	THEN
        	RAISE EXCEPTION E'Cannot delete J:; it is referenced in ALL_CellLine_Derivation(s)';
	END IF;
	IF EXISTS (SELECT * FROM CRS_References
          	WHERE OLD._Object_key = _Refs_key)
	THEN
        	RAISE EXCEPTION E'Cannot delete J:; it is referenced in CRS_References(s)';
	END IF;
	IF EXISTS (SELECT * FROM DAG_DAG
          	WHERE OLD._Object_key = _Refs_key)
	THEN
        	RAISE EXCEPTION E'Cannot delete J:; it is referenced in DAG_DAG(s)';
	END IF;
	IF EXISTS (SELECT * FROM GXD_AntibodyAlias
          	WHERE OLD._Object_key = _Refs_key)
	THEN
        	RAISE EXCEPTION E'Cannot delete J:; it is referenced in GXD_AntibodyAlias(s)';
	END IF;
	IF EXISTS (SELECT * FROM GXD_Assay
          	WHERE OLD._Object_key = _Refs_key)
	THEN
        	RAISE EXCEPTION E'Cannot delete J:; it is referenced in GXD_Assay(s)';
	END IF;
	IF EXISTS (SELECT * FROM GXD_Index
          	WHERE OLD._Object_key = _Refs_key)
	THEN
        	RAISE EXCEPTION E'Cannot delete J:; it is referenced in GXD_Index(s)';
	END IF;
	IF EXISTS (SELECT * FROM IMG_Image
          	WHERE OLD._Object_key = _Refs_key)
	THEN
        	RAISE EXCEPTION E'Cannot delete J:; it is referenced in IMG_Image(s)';
	END IF;
	IF EXISTS (SELECT * FROM MGI_Reference_Assoc
          	WHERE OLD._Object_key = _Refs_key)
	THEN
        	RAISE EXCEPTION E'Cannot delete J:; it is referenced in MGI_Reference_Assoc(s)';
	END IF;
	IF EXISTS (SELECT * FROM MGI_Relationship
          	WHERE OLD._Object_key = _Refs_key)
	THEN
        	RAISE EXCEPTION E'Cannot delete J:; it is referenced in MGI_Relationship(s)';
	END IF;
	IF EXISTS (SELECT * FROM MGI_Synonym
          	WHERE OLD._Object_key = _Refs_key)
	THEN
        	RAISE EXCEPTION E'Cannot delete J:; it is referenced in MGI_Synonym(s)';
	END IF;
	IF EXISTS (SELECT * FROM MLD_Expts
          	WHERE OLD._Object_key = _Refs_key)
	THEN
        	RAISE EXCEPTION E'Cannot delete J:; it is referenced in MLD_Expts(s)';
	END IF;
	IF EXISTS (SELECT * FROM MLD_Notes
          	WHERE OLD._Object_key = _Refs_key)
	THEN
        	RAISE EXCEPTION E'Cannot delete J:; it is referenced in MLD_Notes(s)';
	END IF;
	IF EXISTS (SELECT * FROM MRK_History
          	WHERE OLD._Object_key = _Refs_key)
	THEN
        	RAISE EXCEPTION E'Cannot delete J:; it is referenced in MRK_History(s)';
	END IF;
	IF EXISTS (SELECT * FROM MRK_StrainMarker
          	WHERE OLD._Object_key = _Refs_key)
	THEN
        	RAISE EXCEPTION E'Cannot delete J:; it is referenced in MRK_StrainMarker(s)';
	END IF;
	IF EXISTS (SELECT * FROM PRB_Marker
          	WHERE OLD._Object_key = _Refs_key)
	THEN
        	RAISE EXCEPTION E'Cannot delete J:; it is referenced in PRB_Marker(s)';
	END IF;
	IF EXISTS (SELECT * FROM PRB_Reference
          	WHERE OLD._Object_key = _Refs_key)
	THEN
        	RAISE EXCEPTION E'Cannot delete J:; it is referenced in PRB_Reference(s)';
	END IF;
	IF EXISTS (SELECT * FROM PRB_Source
          	WHERE OLD._Object_key = _Refs_key)
	THEN
        	RAISE EXCEPTION E'Cannot delete J:; it is referenced in PRB_Source(s)';
	END IF;
	IF EXISTS (SELECT * FROM RI_Summary_Expt_Ref
          	WHERE OLD._Object_key = _Refs_key)
	THEN
        	RAISE EXCEPTION E'Cannot delete J:; it is referenced in RI_Summary_Expt_Ref(s)';
	END IF;
	IF EXISTS (SELECT * FROM SEQ_Allele_Assoc
          	WHERE OLD._Object_key = _Refs_key)
	THEN
        	RAISE EXCEPTION E'Cannot delete J:; it is referenced in SEQ_Allele_Assoc(s)';
	END IF;
	IF EXISTS (SELECT * FROM VOC_Evidence
          	WHERE OLD._Object_key = _Refs_key)
	THEN
        	RAISE EXCEPTION E'Cannot delete J:; it is referenced in VOC_Evidence(s)';
	END IF;
	IF EXISTS (SELECT * FROM VOC_Vocab
          	WHERE OLD._Object_key = _Refs_key)
	THEN
        	RAISE EXCEPTION E'Cannot delete J:; it is referenced in VOC_Vocab(s)';
	END IF;
END IF;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.acc_assignj(v_userkey integer, v_objectkey integer, v_nextmgi integer DEFAULT '-1'::integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
j_exists int;
BEGIN
-- NAME: ACC_assignJ
-- DESCRIPTION:
--        
-- To assign a new J: to a Reference
-- 1:Calls ACC_assignMGI() sending prefixPart = 'J:'
-- INPUT:
--      
-- v_userKey   : MGI_User._User_key
-- v_objectKey : ACC_Accession._Object_key
-- v_nextMGI   : if -1, then ACC_assignMGI() will use the next available J:
--		 else, try to use the v_nextMGI as the J: (an override)
-- RETURNS:
--	VOID
--      
IF v_nextMGI != -1
THEN
	j_exists := count(*) FROM BIB_Acc_View
		WHERE prefixPart = 'J:' AND
			_LogicalDB_key = 1 AND
			numericPart = v_nextMGI;
	IF j_exists > 0
	THEN
		RAISE EXCEPTION E'This J Number is already in use: %', j_exists;
		RETURN;
	END IF;
END IF;
-- if this object already has a J:, return/do nothing
IF EXISTS (SELECT 1 FROM ACC_Accession
        WHERE _Object_key = v_objectKey
        AND _LogicalDB_key = 1
        AND _MGIType_key = 1
        AND prefixPart = 'J:')
THEN
        RETURN;
END IF;
PERFORM ACC_assignMGI (v_userKey, v_objectKey, 'Reference', 'J:', v_nextMGI);
RETURN;
END;
$$;


-- CREATE FUNCTION mgd.acc_assignmgi(v_userkey integer, v_objectkey integer, v_mgitype text, v_prefixpart text DEFAULT 'MGI:'::text, v_nextmgi integer DEFAULT '-1'::integer) RETURNS void
--     LANGUAGE plpgsql
--     AS $$
-- DECLARE
-- v_nextACC int;
-- v_mgiTypeKey int;
-- v_accID acc_accession.accid%TYPE;
-- v_rrID acc_accession.accid%TYPE;
-- v_preferred int = 1;
-- v_private int = 0;
-- BEGIN
-- -- NAME: ACC_assignMGI
-- -- DESCRIPTION:
-- --        
-- -- To assign a new MGI id (MGI:xxxx) or new J (J:xxxx)
-- -- Add RRID for Genotypes objects (_MGIType_key = 12)
-- -- Increment ACC_AccessionMax.maxNumericPart
-- -- INPUT:
-- --      
-- -- v_userKey                        : MGI_User._User_key
-- -- v_objectKey                      : ACC_Accession._Object_key
-- -- v_mgiType acc_mgitype.name%TYPE  : ACC_Accession._MGIType_key (MGI: or J:)
-- -- v_prefixPart acc_accession.prefixpart%TYPE : ACC_Accession.prefixPart
-- --		The default is 'MGI:', but 'J:' is also valid
-- -- v_nextMGI int DEFAULT -1 : if -1 then use next available MGI id
-- --		this sp does not allow an override of a generated MGI accession id
-- -- RETURNS:
-- --	VOID
-- --      
-- v_nextACC := max(_Accession_key) + 1 FROM ACC_Accession;
-- v_mgiTypeKey := _MGIType_key FROM ACC_MGIType WHERE name = v_mgiType;
-- IF v_nextMGI = -1
-- THEN
-- 	SELECT INTO v_nextMGI maxNumericPart + 1
-- 	FROM ACC_AccessionMax WHERE prefixPart = v_prefixPart;
-- ELSIF v_prefixPart != 'J:'
-- THEN
--     RAISE EXCEPTION E'Cannot override generation of MGI accession number';
--     RETURN;
-- END IF;
-- -- check if v_nextACC already exists in ACC_Accession table
-- --IF (SELECT count(*) FROM ACC_Accession
-- --    WHERE _MGIType_key = v_mgiTypeKey
-- --    AND prefixPart = v_prefixPart
-- --    AND numericPart = v_nextACC) > 0
-- --THEN
-- --    RAISE EXCEPTION 'v_nextMGI already exists in ACC_Accession: %', v_nextMGI;
-- --    RETURN;
-- --END IF;
-- v_accID := v_prefixPart || v_nextMGI::char(30);
-- INSERT INTO ACC_Accession 
-- (_Accession_key, accID, prefixPart, numericPart, 
-- _LogicalDB_key, _Object_key, _MGIType_key, preferred, private, 
-- _CreatedBy_key, _ModifiedBy_key)
-- VALUES(v_nextACC, v_accID, v_prefixPart, v_nextMGI, 1, v_objectKey, 
-- v_mgiTypeKey, v_preferred, v_private, v_userKey, v_userKey);
-- IF (SELECT maxNumericPart FROM ACC_AccessionMax
--     WHERE prefixPart = v_prefixPart) <= v_nextMGI
-- THEN
-- 	UPDATE ACC_AccessionMax 
-- 	SET maxNumericPart = v_nextMGI 
-- 	WHERE prefixPart = v_prefixPart;
-- END IF;
-- -- add RRID for Genotypes
-- IF v_mgiTypeKey = 12
-- THEN
--     v_nextACC := max(_Accession_key) + 1 FROM ACC_Accession;
--     v_rrID := 'RRID:' || v_accID::char(30); 
--     INSERT INTO ACC_Accession
-- 	(_Accession_key, accID, prefixPart, numericPart,
-- 	_LogicalDB_key, _Object_key, _MGIType_key, preferred, private,
-- 	_CreatedBy_key, _ModifiedBy_key)
-- 	VALUES(v_nextACC, v_rrID, v_prefixPart, v_nextMGI, 179, v_objectKey,
-- 	v_mgiTypeKey, v_preferred, v_private, v_userKey, v_userKey);
-- END IF;
-- RETURN;
-- END;
-- $$;


CREATE FUNCTION mgd.acc_delete_byacckey(v_acckey integer, v_refskey integer DEFAULT '-1'::integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: ACC_delete_byAccKey
-- DESCRIPTION:
--        
-- To delete ACC_Accession and ACC_AccessionReference
-- This may be obsolete now that referential integrity exists
-- INPUT:
--      
-- v_accKey  : ACC_Accession._Accession_key
-- v_refsKey : BIB_Refs._Refs_key
-- RETURNS:
--	VOID
--      
IF v_refsKey = -1
THEN
        DELETE FROM ACC_Accession WHERE _Accession_key = v_accKey;
ELSE
        DELETE FROM ACC_AccessionReference WHERE _Accession_key = v_accKey and _Refs_key = v_refsKey;
        -- If the deletion of the detail would leave the master all alone...
        -- then delete the master too.
        IF NOT EXISTS (SELECT * FROM ACC_AccessionReference WHERE _Accession_key = v_accKey)
        THEN
            DELETE FROM ACC_Accession WHERE _Accession_key = v_accKey;
        END IF;
END IF;
END;
$$;


-- CREATE FUNCTION mgd.acc_insert(v_userkey integer, v_objectkey integer, v_accid text, v_logicaldb integer, v_mgitype text, v_refskey integer DEFAULT '-1'::integer, v_preferred integer DEFAULT 1, v_private integer DEFAULT 0, v_dosplit integer DEFAULT 1) RETURNS void
--     LANGUAGE plpgsql
--     AS $$
-- DECLARE
-- v_nextACC int;
-- v_mgiTypeKey int;
-- v_prefixPart acc_accession.prefixPart%TYPE;
-- v_numericPart acc_accession.prefixPart%Type;
-- BEGIN
-- -- NAME: ACC_insert
-- -- DESCRIPTION:
-- --        
-- -- To add new ACC_Accession record with out checking Accession id rules
-- -- If reference is given, insert record into ACC_AccessionReference
-- -- INPUT:
-- --      
-- -- v_userKey                        : MGI_User._User_key
-- -- v_objectKey                      : ACC_Accession._Object_key
-- -- v_accID acc_accession.accid%TYPE : ACC_Accession.accID
-- -- v_logicalDB                      : ACC_Accession._LogicalDB_key
-- -- v_mgiType acc_mgitype.name%TYPE  : ACC_Accession._MGIType_key
-- -- v_refsKey int DEFAULT -1         : BIB_Refs._Refs_key; if Reference, then call ACCRef_insert()
-- -- v_preferred int DEFAULT 1        : ACC_Accession.prefixPart
-- -- v_private int DEFAULT 0          : ACC_Accession.private
-- -- v_dosplit int DEFAULT 1          : if 1, split the accession id into prefixpart/numericpart
-- -- RETURNS:
-- --      
-- IF v_accID IS NULL
-- THEN
-- 	RETURN;
-- END IF;
-- v_nextACC := max(_Accession_key) + 1 FROM ACC_Accession;
-- v_mgiTypeKey := _MGIType_key FROM ACC_MGIType WHERE name = v_mgiType;
-- v_prefixPart := v_accID;
-- v_numericPart := '';
-- -- skip the splitting...for example, the Reference/DOI ids are not split
-- IF v_dosplit
-- THEN
-- 	-- split accession id INTO prefixPart/numericPart
-- 	SELECT * FROM ACC_split(v_accID) INTO v_prefixPart, v_numericPart;
-- END IF;
-- IF (select count(accid) from acc_accession 
--         where _mgitype_key = 10 and _logicaldb_key = v_logicalDB and accid = v_accID
--         group by _logicaldb_key, accid having count(*) >= 1)
-- THEN
--         RAISE EXCEPTION E'Cannot assign same Registry:Accession Id to > 1 Strain';
--         RETURN;
-- END IF;
-- IF v_numericPart = ''
-- THEN
-- 	INSERT INTO ACC_Accession
-- 	(_Accession_key, accID, prefixPart, numericPart, _LogicalDB_key, _Object_key, 
--  	 _MGIType_key, private, preferred, _CreatedBy_key, _ModifiedBy_key)
-- 	VALUES(v_nextACC, v_accID, v_prefixPart, null, 
--                v_logicalDB, v_objectKey, v_mgiTypeKey, v_private, v_preferred, v_userKey, v_userKey);
-- ELSE
-- 	INSERT INTO ACC_Accession
-- 	(_Accession_key, accID, prefixPart, numericPart, _LogicalDB_key, _Object_key, 
--  	 _MGIType_key, private, preferred, _CreatedBy_key, _ModifiedBy_key)
-- 	VALUES(v_nextACC, v_accID, v_prefixPart, v_numericPart::integer, 
--                v_logicalDB, v_objectKey, v_mgiTypeKey, v_private, v_preferred, v_userKey, v_userKey);
-- END IF;
-- IF v_refsKey != -1
-- THEN
-- 	PERFORM ACCRef_insert(v_userKey, v_nextAcc, v_refsKey);
-- END IF;
-- RETURN;
-- END;
-- $$;


CREATE FUNCTION mgd.acc_setmax(v_increment integer, v_prefixpart text DEFAULT 'MGI:'::text) RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: ACC_setMax
-- DESCRIPTION:
--        
-- To update/set the ACC_AccessionMax.maxNumericPart for given type
-- INPUT:
--      
-- v_increment : the amount to which to increment the maxNumericPart
-- v_prefixPart acc_accession.prefixPart%TYPE DEFAULT 'MGI:'
--	the default is 'MGI:' type
--      else enter a valid ACC_Accession.prefixPart ('J:')
-- RETURNS:
--	VOID
--      
UPDATE ACC_AccessionMax
SET maxNumericPart = maxNumericPart + v_increment
WHERE prefixPart = v_prefixPart
;
END;
$$;


CREATE FUNCTION mgd.acc_split(v_accid text, OUT v_prefixpart text, OUT v_numericpart text) RETURNS record
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: ACC_split
-- DESCRIPTION:
--        
-- To split an Accession ID into a prefixPart and a numericPart
-- to call : select v_prefixPart, v_numericPart from ACC_split('MGI:12345');
-- example : A2ALT2 -> (A2ALT, 2)
-- example : MGI:12345 -> (MGI:, 12345)
-- INPUT:
--      
-- v_accID acc_accession.accid%TYPE                : ACC_Accession.accID
-- RETURNS:
--	prefixPart  : ACC_Accession.prefixPart%TYPE
--      numericPart : ACC_Accession.numericPart%TYPE
--      
-- v_prefixPart is alphanumeric
v_prefixPart := (SELECT (regexp_matches(v_accID, E'^((.*[^0-9])?)([0-9]*)', 'g'))[2]);
-- v_numericPart is numeric only
v_numericPart := (SELECT (regexp_matches(v_accID, E'^((.*[^0-9])?)([0-9]*)', 'g'))[3]);
RETURN;
END;
$$;


-- CREATE FUNCTION mgd.acc_update(v_userkey integer, v_acckey integer, v_accid text, v_private integer DEFAULT 0, v_origrefskey integer DEFAULT '-1'::integer, v_refskey integer DEFAULT '-1'::integer) RETURNS void
--     LANGUAGE plpgsql
--     AS $$
-- DECLARE
-- v_prefixPart acc_accession.prefixPart%TYPE;
-- v_numericPart acc_accession.prefixPart%TYPE;
-- BEGIN
-- -- NAME: ACC_update
-- -- DESCRIPTION:
-- --        
-- -- To update an ACC_Accession, ACC_AccessionReference records
-- -- INPUT:
-- --      
-- -- v_userKey     : MGI_User._User_key
-- -- v_accKey      : ACC_Accession._Accession_key
-- -- v_accID       : ACC_Accession.accID
-- -- v_origRefsKey : original ACC_AccessionReference._Refs_key
-- -- v_refsKey     : new ACC_AccessionReference._Refs_key
-- -- v_private     : ACC_Accessioin.private
-- -- RETURNS:
-- --	VOID
-- --      
-- v_numericPart := '';
-- IF v_accID IS NULL
-- THEN
-- 	select ACC_delete_byAccKey (v_accKey);
-- ELSE
--         -- split accession id INTO prefixPart/numericPart
--         SELECT * FROM ACC_split(v_accID) INTO v_prefixPart, v_numericPart;
-- 	IF (v_prefixPart = 'J:' or substring(v_prefixPart,1,4) = 'MGD-')
-- 	THEN
-- 		IF (select count(*) from ACC_Accession
-- 		    where numericPart = v_numericPart::integer
-- 			  and prefixPart = v_prefixPart) >= 1
-- 		THEN
--                         RAISE EXCEPTION E'Duplicate MGI Accession Number';
--                         RETURN;
-- 		END IF;
-- 	END IF;
-- 	IF v_numericPart = ''
-- 	THEN
-- 		update ACC_Accession
-- 	       	set accID = v_accID,
-- 		   	prefixPart = v_prefixPart,
-- 		   	numericPart = null,
--                         private = v_private,
-- 		   	_ModifiedBy_key = v_userKey,
-- 		   	modification_date = now()
-- 	       	where _Accession_key = v_accKey;
--         ELSE
-- 		update ACC_Accession
-- 	       	set accID = v_accID,
-- 		   	prefixPart = v_prefixPart,
-- 		   	numericPart = v_numericPart::integer,
--                         private = v_private,
-- 		   	_ModifiedBy_key = v_userKey,
-- 		   	modification_date = now()
-- 	       	where _Accession_key = v_accKey;
-- 	END IF;
-- 	IF v_refsKey > 0
-- 	THEN
-- 		update ACC_AccessionReference
-- 		       set _Refs_key = v_refsKey,
-- 		           _ModifiedBy_key = v_userKey,
-- 		           modification_date = now()
-- 		           where _Accession_key = v_accKey and _Refs_key = v_origRefsKey;
-- 	END IF;
-- END IF;
-- END;
-- $$;


CREATE FUNCTION mgd.accref_insert(v_userkey integer, v_acckey integer, v_refskey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: ACCRef_insert
-- DESCRIPTION:
--    
-- To insert a new reference record into ACC_AccessionReference
-- INPUT:
--      
-- v_userKey : MGI_User._User_key
-- v_accKey  : ACC_Accession._Accession_key
-- v_refsKey : BIB_Refs._Refs_key
-- RETURNS:
--	VOID
--      
-- Insert record into ACC_AccessionReference table
INSERT INTO ACC_AccessionReference
(_Accession_key, _Refs_key, _CreatedBy_key, _ModifiedBy_key)
VALUES(v_accKey, v_refsKey, v_userKey, v_userKey)
;
END;
$$;


CREATE FUNCTION mgd.accref_process(v_userkey integer, v_objectkey integer, v_refskey integer, v_accid text, v_logicaldb integer, v_mgitype text, v_preferred integer DEFAULT 1, v_private integer DEFAULT 0) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_accKey int;
BEGIN
-- NAME: ACCRef_process
-- DESCRIPTION:
--        
-- To add a new Accession/Reference row to ACC_AccessionReference
-- If an Accession object already exists, then call ACCRef_insert() to add the new Accession/Reference row
-- Else, call ACC_insert() to add a new Accession object and a new Accession/Reference row
-- INPUT:
--      
-- v_userKey                        : MGI_User._User_key
-- v_objectKey                      : ACC_Accession._Object_key
-- v_refsKey                        : BIB_Refs._Refs_key
-- v_accID acc_accession.accid%TYPE : ACC_Accession.accID
-- v_logicalDB                      : ACC_Accession._LogicalDB_key
-- v_mgiType acc_mgitype.name%TYPE  : ACC_Accession._MGIType_key
-- v_preferred int DEFAULT 1        : ACC_Accession.prefixPart
-- v_private int DEFAULT 0          : ACC_Accession.private
-- RETURNS:
--	VOID
--      
-- If the Object/Acc ID pair already exists, then use that _Accession_key
-- and simply insert a new ACC_AccessionReference record (ACCRef_insert)
-- Else, create a new ACC_Accession and ACC_AccessionReference record (ACC_insert)
SELECT INTO v_accKey a._Accession_key 
FROM ACC_Accession a, ACC_MGIType m
WHERE a.accID = v_accID
AND a._Object_key = v_objectKey
AND a._MGIType_key = m._MGIType_key
AND m.name = v_mgiType
AND a._LogicalDB_key = v_logicalDB
;
IF v_accKey IS NOT NULL
THEN
	PERFORM ACCRef_insert (v_userKey, v_accKey, v_refsKey);
ELSE
	PERFORM ACC_insert (v_userKey, v_objectKey, v_accID, v_logicalDB, v_mgiType, v_refsKey, v_preferred, v_private);
END IF;
RETURN;
END;
$$;


CREATE FUNCTION mgd.all_allele_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: ALL_Allele_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INPUT:
--	none
-- RETURNS:
--	NEW
-- NOTE:  Any changes should also be reflected in:
--     pgdbutilities/sp/MGI_deletePrivateData.csh
IF EXISTS (SELECT * FROM VOC_Annot a
          WHERE OLD._Allele_key = a._Object_key
          AND a._AnnotType_key = 1021)
THEN
        RAISE EXCEPTION E'Allele is referenced in Allele/Disease Annotations';
END IF;
-- update cache tables
UPDATE GXD_AlleleGenotype
SET _Marker_key = null
WHERE _Marker_key = OLD._Marker_key
AND GXD_AlleleGenotype._Allele_key = OLD._Allele_key
;
UPDATE GXD_AllelePair
SET _Marker_key = null
WHERE _Marker_key = OLD._Marker_key
AND (GXD_AllelePair._Allele_key_1 = OLD._Allele_key or GXD_AllelePair._Allele_key_2 = OLD._Allele_key)
;
UPDATE PRB_Strain_Marker
SET _Marker_key = null
WHERE _Marker_key = OLD._Marker_key
AND PRB_Strain_Marker._Allele_key = OLD._Allele_key
;
DELETE FROM IMG_ImagePane_Assoc a
WHERE a._Object_key = OLD._Allele_key
AND a._MGIType_key = 11
;
DELETE FROM MAP_Coord_Feature a
WHERE a._Object_key = OLD._Allele_key
AND a._MGIType_key = 11
;
DELETE FROM MAP_Coordinate a
WHERE a._Object_key = OLD._Allele_key
AND a._MGIType_key = 11
;
DELETE FROM MGI_Note a
WHERE a._Object_key = OLD._Allele_key
AND a._MGIType_key = 11
;
DELETE FROM MGI_Property a
WHERE a._Object_key = OLD._Allele_key
AND a._MGIType_key = 11
;
DELETE FROM MGI_Reference_Assoc a
WHERE a._Object_key = OLD._Allele_key
AND a._MGIType_key = 11
;
DELETE FROM MGI_Relationship a
WHERE a._Object_key_1 = OLD._Allele_key
AND a._Category_key in (1003, 1004, 1006)
;
DELETE FROM MGI_Synonym a
WHERE a._Object_key = OLD._Allele_key
AND a._MGIType_key = 11
;
DELETE FROM VOC_Annot a
WHERE a._Object_key = OLD._Allele_key
AND a._AnnotType_key = 1014
;
DELETE FROM ACC_Accession a
WHERE a._Object_key = OLD._Allele_key
AND a._MGIType_key = 11
;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.all_allele_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: ALL_Allele_insert()
-- DESCRIPTOIN:
--	this insert trigger will call ACC_assignMGI
--	in order to add a distinct MGI accession id
--	to the NEW object
-- RULES:
-- INPUT:
--	none
-- RETURNS:
--	NEW
PERFORM ACC_assignMGI(1001, NEW._Allele_key, 'Allele');
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.all_allele_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: ALL_Allele_update()
-- NAME: ALL_Allele_update2()
-- DESCRIPTOIN:
-- 	1) GO check: approved/autoload may exist in VOC_Evidence/inferredFrom
--	2) if setting Allele Status = Deleted, then check cross-references
-- RULES:
-- INPUT:
--	none
-- RETURNS:
--	NEW
-- in progress =  847111
-- reserved = 847113
-- approved = 847114
-- autoload = 3983021
-- deleted =  847112
-- GO check: approved/autoload
IF OLD._Allele_Status_key in (847114,3983021)
   AND
   NEW._Allele_Status_key in (847111,847112,847113)
   AND
   EXISTS (SELECT 1 FROM VOC_Annot a, VOC_Evidence e, ACC_Accession aa
        WHERE a._AnnotType_key = 1000
        AND a._Object_key = OLD._Marker_key
        AND a._Annot_key = e._Annot_key
        AND e.inferredFrom = aa.accID
        AND aa._MGIType_key = 11
        AND aa._Object_key = OLD._Allele_key)
THEN
        RAISE EXCEPTION E'You cannot change the Allele status from\nApproved/Autoload to Deleted, Reserved or Pending.\n\nThis Allele is cross-referenced to a GO annotation.';
END IF;
-- if setting Allele Status = Deleted, then check cross-references
IF NEW._Allele_Status_key = 847112
   AND
   EXISTS (SELECT 1 FROM MGI_Relationship r
        WHERE r._Category_key in (1003, 1004, 1006)
        AND r._Object_key_1 = NEW._Allele_key)
THEN
        RAISE EXCEPTION E'You cannot change the Allele status to\nDeleted.\n\nThis Allele is cross-referenced to a Organizer in a MGI_Relationship record.';
END IF;
-- Non-Approved Alleles cannot have Genotypes or Strain associations
IF OLD._Allele_Status_key != NEW._Allele_Status_key 
	AND NEW._Allele_Status_key NOT IN (847114,3983021)
THEN
    IF EXISTS (SELECT * FROM GXD_AlleleGenotype WHERE GXD_AlleleGenotype._Allele_key = NEW._Allele_key)
    THEN
            RAISE EXCEPTION E'Allele Symbol is referenced in GXD Allele Pair Record(s); Approved/Autoload Status cannot be changed.';
    END IF;
    IF EXISTS (SELECT * FROM MLD_Expt_Marker
            WHERE MLD_Expt_Marker._Allele_key = NEW._Allele_key)
    THEN
            RAISE EXCEPTION E'Allele Symbol is referenced in Mapping Experiment Marker Record(s); Approved/Autoload Status cannot be changed.';
    END IF;
    IF EXISTS (SELECT * FROM PRB_Strain_Marker
            WHERE PRB_Strain_Marker._Allele_key = NEW._Allele_key)
    THEN
            RAISE EXCEPTION E'Allele Symbol is referenced in Strain/Allele Record(s); Approved/Autoload Status cannot be changed.';
    END IF;
END IF;
-- update cache tables
UPDATE GXD_AlleleGenotype
set _Marker_key = NEW._Marker_key
WHERE _Allele_key = NEW._Allele_key
AND NEW._MarkerAllele_Status_key != 4268546
;
UPDATE GXD_AllelePair
set _Marker_key = NEW._Marker_key
WHERE (_Allele_key_1 = NEW._Allele_key or _Allele_key_2 = NEW._Allele_key)
AND NEW._MarkerAllele_Status_key != 4268546
;
UPDATE PRB_Strain_Marker
set _Marker_key = NEW._Marker_key
WHERE _Allele_key = NEW._Allele_key
AND NEW._MarkerAllele_Status_key != 4268546
;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.all_cellline_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: ALL_CellLine_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INPUT:
--	none
-- RETURNS:
--	NEW
DELETE FROM IMG_ImagePane_Assoc a
WHERE a._Object_key = OLD._CellLine_key
AND a._MGIType_key = 28
;
DELETE FROM MAP_Coord_Feature a
WHERE a._Object_key = OLD._CellLine_key
AND a._MGIType_key = 28
;
DELETE FROM MAP_Coordinate a
WHERE a._Object_key = OLD._CellLine_key
AND a._MGIType_key = 28
;
DELETE FROM MGI_Note a
WHERE a._Object_key = OLD._CellLine_key
AND a._MGIType_key = 28
;
DELETE FROM MGI_Property a
WHERE a._Object_key = OLD._CellLine_key
AND a._MGIType_key = 28
;
DELETE FROM MGI_Reference_Assoc a
WHERE a._Object_key = OLD._CellLine_key
AND a._MGIType_key = 28
;
DELETE FROM MGI_Synonym a
WHERE a._Object_key = OLD._CellLine_key
AND a._MGIType_key = 28
;
DELETE FROM ACC_Accession a
WHERE a._Object_key = OLD._CellLine_key
AND a._MGIType_key = 28
;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.all_cellline_update1() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: ALL_CellLine_update1()
-- NAME: ALL_CellLine_update2()
-- DESCRIPTOIN:
-- INPUT:
--	none
-- RETURNS:
--	NEW
-- update strains of mutant cell lines derived from this parent line
UPDATE ALL_CellLine
SET _Strain_key = NEW._Strain_key
FROM ALL_CellLine_Derivation d
WHERE NEW._CellLine_key = d._ParentCellLine_key
AND d._Derivation_key = ALL_CellLine._Derivation_key
;
-- Previously, when Strain was updated, we also needed to update all
-- Alleles which reference this ES Cell Line AND the old Strain (excluding
-- NA, NS, Other (see notes) cell lines).  Now, that is done by a nightly
-- script rather than in this trigger.
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.all_cellline_update2() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- If a mutant cell line has a change in its derivation key, then its strain
-- key must change to match the strain of its new parent cell line.
UPDATE ALL_CellLine
SET _Strain_key = p._Strain_key
FROM ALL_CellLine_Derivation d, ALL_CellLine p
WHERE NEW._CellLine_key = ALL_CellLine._CellLine_key
AND ALL_CellLine._Derivation_key = d._Derivation_key
AND d._ParentCellLine_key = p._CellLine_key
;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.all_convertallele(v_userkey integer, v_markerkey integer, v_oldsymbol text, v_newsymbol text, v_alleleof integer DEFAULT 0) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_alleleKey int;
BEGIN
-- NAME: ALL_convertAllele
-- DESCRIPTION:
--        
-- Convert allele symbols of v_markerKey using v_oldSymbol and v_newSymbol values;
-- do NOT call this with a null value of v_markerKey, or it will fail and roll
-- back your transaction.
-- INPUT:
--      
-- v_userKey   : MGI_User._User_key
-- v_markerKey : MRK_Marker._Marker_key
-- v_oldSymbol mrk_marker.symbol%TYPE : old marker symbol
-- v_newSymbol mrk_marker.symbol%TYPE : new marker symbol
-- v_alleleOf int DEFAULT 0 : is old Symbol an Allele of the new Symbol?
-- RETURNS:
--	VOID
--      
-- If the marker key is null, then this procedure will not work.  Bail out
-- now before anything gets mixed up.
IF v_markerKey IS NULL
THEN
	RAISE EXCEPTION E'ALL_convertAllele : Cannot update symbols for allele which have no marker.';
	RETURN;
END IF;
-- If old Symbol is NOT allele of new Symbol... 
-- Convert new alleles:  
--	oldallele<+> --> newsymbol<+> 
--	oldallele<allele> --> newsymbol<allele> 
--      oldallele         --> newsymbol         
IF v_alleleOf = 0
THEN
	--	oldallele<+> --> newsymbol<+> 
	--	oldallele<allele> --> newsymbol<allele> 
	update ALL_Allele
	set symbol = v_newSymbol || '<' ||
		substring(symbol,  position('<' in symbol) + 1, char_length(symbol)),
		_ModifiedBy_key = v_userKey, modification_date = now()
	where _Marker_key = v_markerKey and symbol like '%<%'
	;
	--      oldallele         --> newsymbol         
	update ALL_Allele set symbol = v_newSymbol, 
		_ModifiedBy_key = v_userKey, modification_date = now()
	where _Marker_key = v_markerKey and symbol = v_oldSymbol
	;
ELSE
	-- If old Symbol is an Allele of new Symbol... 
	-- Convert new alleles:  
	--	oldallele<+> --> newsymbol<+> 
	--	oldallele<allele> --> newsymbol<oldallele-allele> 
	--      oldallele         --> newsymbol<oldallele>        
	-- Non Wild Type 
	--	oldallele<allele> --> newsymbol<oldallele-allele> 
	update ALL_Allele
	set symbol = v_newSymbol || '<' || 
	        substring(symbol, 1, position('<' in symbol) - 1) || '-' ||
		substring(symbol,  position('<' in symbol) + 1, char_length(symbol)),
		_ModifiedBy_key = v_userKey, modification_date = now()
	where _Marker_key = v_markerKey and symbol like '%<%' and isWildType = 0
	;
	--      oldallele         --> newsymbol<oldallele>        
	update ALL_Allele
	set symbol = v_newSymbol || '<' || symbol || '>', 
		_ModifiedBy_key = v_userKey, modification_date = now()
	where _Marker_key = v_markerKey and symbol not like '%<%' and isWildType = 0
	;
	-- Wild Type Allele 
	--	oldsymbol<+>	  --> newsymbol<+>        
	update ALL_Allele
	set symbol = v_newSymbol || '<+>',
		_ModifiedBy_key = v_userKey, modification_date = now()
	where _Marker_key = v_markerKey and isWildType = 1 and symbol like '%<+>'
	;
END IF;
FOR v_alleleKey IN
SELECT _Allele_key from ALL_Allele where _Marker_key = v_markerKey
LOOP
	PERFORM ALL_reloadLabel (v_alleleKey);
END LOOP;
END;
$$;


-- CREATE FUNCTION mgd.all_createwildtype(v_userkey integer, v_markerkey integer, v_symbol text) RETURNS void
--     LANGUAGE plpgsql
--     AS $$
-- DECLARE
-- v_asymbol all_allele.symbol%TYPE;
-- BEGIN
-- -- NAME: ALL_createWildType
-- -- DESCRIPTION:
-- --        
-- -- Create a Wild Type Allele
-- -- Use Reference = J:23000
-- -- Set all other attributes = Not Applicable
-- -- INPUT:
-- -- v_userKey   : MGI_User._User_key
-- -- v_markerKey : MRK_Marker._Marker_key
-- -- v_symbol mrk_marker.symbol%TYPE
-- -- RETURNS:
-- --	VOID
-- --      
-- v_asymbol := v_symbol || '<+>';
-- PERFORM ALL_insertAllele (
-- 	v_userKey,
-- 	v_markerKey,
-- 	v_asymbol,
-- 	'wild type',
-- 	null,
-- 	1,
-- 	v_userKey,
-- 	v_userKey,
-- 	v_userKey,
-- 	current_date,
-- 	-2,
-- 	'Not Applicable',
-- 	'Not Applicable',
--         'Approved',
--         null,
--         'Not Applicable',
--         'Not Specified',
--         'Not Specified',
--         'Curated'
-- 	);
-- IF NOT FOUND
-- THEN
-- 	RAISE EXCEPTION E'ALL_createWildType: PERFORM ALL_insertAllele failed';
-- END IF;
-- END;
-- $$;


-- CREATE FUNCTION mgd.all_insertallele(v_userkey integer, v_markerkey integer, v_symbol text, v_name text, v_refskey integer DEFAULT NULL::integer, v_iswildtype integer DEFAULT 0, v_createdby integer DEFAULT 1001, v_modifiedby integer DEFAULT 1001, v_approvedby integer DEFAULT NULL::integer, v_approval_date timestamp without time zone DEFAULT NULL::timestamp without time zone, v_strainkey integer DEFAULT '-1'::integer, v_amode text DEFAULT 'Not Specified'::text, v_atype text DEFAULT 'Not Specified'::text, v_astatus text DEFAULT 'Approved'::text, v_oldsymbol text DEFAULT NULL::text, v_transmission text DEFAULT 'Not Applicable'::text, v_collection text DEFAULT 'Not Specified'::text, v_qualifier text DEFAULT 'Not Specified'::text, v_mrkstatus text DEFAULT 'Curated'::text) RETURNS void
--     LANGUAGE plpgsql
--     AS $$
-- DECLARE
-- v_alleleKey int;
-- v_createdByKey int;
-- v_modifiedByKey int;
-- v_approvedByKey int;
-- v_modeKey int;
-- v_typeKey int;
-- v_statusKey int;
-- v_transmissionKey int;
-- v_collectionKey int;
-- v_assocKey int;
-- v_qualifierKey int;
-- v_mrkstatusKey int;
-- v_isExtinct all_allele.isextinct%TYPE;
-- v_isMixed all_allele.ismixed%TYPE;
-- BEGIN
-- -- NAME: ALL_insertAllele
-- -- DESCRIPTION:
-- --        
-- -- To insert a new Allele into ALL_Allele
-- -- Also calls: MGI_insertReferenceAssoc() to create 'Original'/1011 reference
-- -- INPUT:
-- --      
-- -- see below
-- -- RETURNS:
-- --	VOID
-- --      
-- v_alleleKey := nextval('all_allele_seq');
-- IF v_createdBy IS NULL
-- THEN
-- 	v_createdByKey := v_userKey;
-- ELSE
-- 	v_createdByKey := _User_key from MGI_User where login = current_user;
-- END IF;
-- IF v_modifiedBy IS NULL
-- THEN
-- 	v_modifiedByKey := v_userKey;
-- ELSE
-- 	v_modifiedByKey := _User_key from MGI_User where login = current_user;
-- END IF;
-- IF v_approvedBy IS NULL
-- THEN
-- 	v_approvedByKey := v_userKey;
-- ELSE
-- 	v_approvedByKey := _User_key from MGI_User where login = current_user;
-- END IF;
-- v_modeKey := _Term_key from VOC_Term where _Vocab_key = 35 AND term = v_amode;
-- v_typeKey := _Term_key from VOC_Term where _Vocab_key = 38 AND term = v_atype;
-- v_statusKey := _Term_key from VOC_Term where _Vocab_key = 37 AND term = v_astatus;
-- v_transmissionKey := _Term_key from VOC_Term where _Vocab_key = 61 AND term = v_transmission;
-- v_collectionKey := _Term_key from VOC_Term where _Vocab_key = 92 AND term = v_collection;
-- v_qualifierKey := _Term_key from VOC_Term where _Vocab_key = 70 AND term = v_qualifier;
-- v_mrkstatusKey := _Term_key from VOC_Term where _Vocab_key = 73 AND term = v_mrkstatus;
-- v_isExtinct := 0;
-- v_isMixed := 0;
-- IF v_astatus = 'Approved' AND v_approval_date IS NULL
-- THEN
-- 	v_approval_date := now();
-- END IF;
-- /* Insert New Allele into ALL_Allele */
-- INSERT INTO ALL_Allele
-- (_Allele_key, _Marker_key, _Strain_key, _Mode_key, _Allele_Type_key, _Allele_Status_key, _Transmission_key,
--         _Collection_key, symbol, name, isWildType, isExtinct, isMixed,
-- 	_Refs_key, _MarkerAllele_Status_key,
--         _CreatedBy_key, _ModifiedBy_key, _ApprovedBy_key, approval_date, creation_date, modification_date)
-- VALUES(v_alleleKey, v_markerKey, v_strainKey, v_modeKey, v_typeKey, v_statusKey, v_transmissionKey,
--         v_collectionKey, v_symbol, v_name, v_isWildType, v_isExtinct, v_isMixed,
-- 	v_refsKey, v_mrkstatusKey,
--         v_createdByKey, v_modifiedByKey, v_approvedByKey, v_approval_date, now(), now())
-- ;
-- IF v_refsKey IS NOT NULL
-- THEN
--         PERFORM MGI_insertReferenceAssoc (v_userKey, 11, v_alleleKey, v_refsKey, 1011);
-- END IF;
-- --IF (v_oldSymbol IS NOT NULL) AND (v_markerKey IS NOT NULL)
-- --THEN
--         --UPDATE MLD_Expt_Marker SET _Allele_key = v_alleleKey 
-- 	--WHERE _Marker_key = v_markerKey AND gene = v_oldSymbol;
-- --END IF;
-- END;
-- $$;


CREATE FUNCTION mgd.all_mergeallele(v_oldallelekey integer, v_newallelekey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: ALL_mergeAllele
-- DESCRIPTION:
--        
-- To re-set _Allele_key from old _Allele_key to new _Allele_key
-- called from ALL_mergeWildTypes()
-- INPUT:
--      
-- v_oldAlleleKey : ALL_Allele._Allele_key (old)
-- v_newAlleleKey : ALL_Allele._Allele_key (new)
-- RETURNS:
--	VOID
--      
UPDATE ALL_Knockout_Cache SET _Allele_key = v_newAlleleKey WHERE _Allele_key = v_oldAlleleKey
;
UPDATE GXD_AlleleGenotype SET _Allele_key = v_newAlleleKey WHERE _Allele_key = v_oldAlleleKey
;
UPDATE GXD_AllelePair SET _Allele_key_1 = v_newAlleleKey WHERE _Allele_key_1 = v_oldAlleleKey
;
UPDATE GXD_AllelePair SET _Allele_key_2 = v_newAlleleKey WHERE _Allele_key_2 = v_oldAlleleKey
;
UPDATE MLD_Expt_Marker SET _Allele_key = v_newAlleleKey WHERE _Allele_key = v_oldAlleleKey
;
DELETE FROM ALL_Allele WHERE _Allele_key = v_oldAlleleKey
;
END;
$$;


CREATE FUNCTION mgd.all_mergewildtypes(v_oldkey integer, v_newkey integer, v_oldsymbol text, v_newsymbol text) RETURNS void
    LANGUAGE plpgsql
    AS $$
-- do NOT call this procedure with one or both of the marker keys being null,
-- or it will fail and will roll back your transaction
DECLARE
v_oldAlleleKey int;
v_newAlleleKey int;
BEGIN
-- NAME: ALL_mergeWildTypes
-- DESCRIPTION:
--        
-- To merge wild type alleles : calls ALL_mergeAllele()
-- exception if oldSymbol or newSymbol is null
-- INPUT:
-- v_oldKey : ALL_Allele._Allele_key
-- v_newKey : ALL_Allele._Allele_key
-- v_oldSymbol mrk_marker.symbol%TYPE
-- v_newSymbol mrk_marker.symbol%TYPE
--      
-- RETURNS:
--	VOID
--      
IF (v_oldKey IS NULL) OR (v_newKey IS NULL)
THEN
	RAISE EXCEPTION E'ALL_mergeWildTypes : Cannot merge wild types if a marker key is null';
	RETURN;
END IF;
v_oldAlleleKey := _Allele_key from ALL_Allele where _Marker_key = v_oldKey and isWildType = 1;
v_newAlleleKey := _Allele_key from ALL_Allele where _Marker_key = v_newKey and isWildType = 1;
IF v_oldAlleleKey IS NOT NULL and v_newAlleleKey IS NOT NULL
THEN
	PERFORM ALL_mergeAllele (v_oldAlleleKey, v_newAlleleKey);
	IF NOT FOUND
	THEN
		RAISE EXCEPTION E'ALL_mergeWildTypes : Cannot execute ALL_mergeAllele()';
		RETURN;
	END IF;
END IF;
END;
$$;


-- CREATE FUNCTION mgd.all_reloadlabel(v_allelekey integer) RETURNS void
--     LANGUAGE plpgsql
--     AS $$
-- DECLARE
-- v_userKey int;
-- v_labelstatusKey int;
-- v_priority int;
-- v_label all_label.label%TYPE;
-- v_labelType all_label.labelType%TYPE;
-- v_labelTypeName all_label.labelTypeName%TYPE;
-- BEGIN
-- -- NAME: ALL_reloadLabel
-- -- DESCRIPTION:
-- --        
-- -- reload ALL_Label for given Allele
-- -- INPUT:
-- --      
-- -- v_alleleKey : ALL_Allele._Allele_key
-- -- RETURNS:
-- --	VOID
-- --      
-- -- Delete all ALL_Label records for a Allele and regenerate
-- DELETE FROM ALL_Label WHERE _Allele_key = v_alleleKey;
-- FOR v_labelstatusKey, v_priority, v_label, v_labelType, v_labelTypeName IN
-- SELECT DISTINCT 1 as _Label_Status_key, 1 as priority, 
-- a.symbol, 'AS' as labelType, 'allele symbol' as labelTypeName
-- FROM ALL_Allele a
-- WHERE a._Allele_key = v_alleleKey
-- AND a.isWildType = 0
-- UNION 
-- SELECT distinct 1 as _Label_Status_key, 2 as priority,
-- a.name, 'AN' as labelType, 'allele name' as labelTypeName
-- FROM ALL_Allele a
-- WHERE a._Allele_key = v_alleleKey
-- AND a.isWildType = 0
-- UNION
-- SELECT 1 as _Label_Status_key, 3 as priority,
-- s.synonym, 'AY' as labelType, 'synonym' as labelTypeName
-- FROM MGI_Synonym_Allele_View s
-- WHERE s._Object_key = v_alleleKey
-- LOOP
-- 	INSERT INTO ALL_Label 
-- 	(_Allele_key, _Label_Status_key, priority, label, labelType, labelTypeName, creation_date, modification_date)
-- 	VALUES (v_alleleKey, v_labelstatusKey, v_priority, v_label, v_labelType, v_labelTypeName, now(), now())
-- 	;
-- END LOOP;
-- END;
-- $$;


CREATE FUNCTION mgd.all_variant_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: ALL_Variant_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INPUT:
--	none
-- RETURNS:
--	NEW
-- NOTE:  Any changes should also be reflected in:
--     pgdbutilities/sp/MGI_deletePrivateData.csh
DELETE FROM MGI_Note a
WHERE a._Object_key = OLD._Variant_key
AND a._MGIType_key = 45
;
DELETE FROM MGI_Reference_Assoc a
WHERE a._Object_key = OLD._Variant_key
AND a._MGIType_key = 45
;
DELETE FROM VOC_Annot a
WHERE a._Object_key = OLD._Variant_key
AND a._AnnotType_key in (1026, 1027)
;
DELETE FROM ACC_Accession a
WHERE a._Object_key = OLD._Variant_key
AND a._MGIType_key = 45
;
-- delete the source variant as well
IF (OLD._SourceVariant_key is not null)
THEN
	DELETE FROM ALL_Variant a
	WHERE a._Variant_key = OLD._SourceVariant_key
	;
END IF;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.bib_keepwfrelevance(v_refskey integer, v_userkey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
 
BEGIN
-- NAME: BIB_keepWFRelevance
-- DESCRIPTION:
--        
-- To set the current Workflow Relevance = keep, if it does not already exist
-- INPUT:
--	None
-- RETURNS:
--	VOID
--      
IF (SELECT count(*) FROM BIB_Workflow_Relevance WHERE isCurrent = 1 AND _Relevance_key != 70594667 AND _Refs_key = v_refsKey) > 0
THEN
    UPDATE BIB_Workflow_Relevance w set isCurrent = 0 
    WHERE _Refs_key = v_refsKey
    ;   
    INSERT INTO BIB_Workflow_Relevance 
    VALUES((select nextval('bib_workflow_relevance_seq')), v_refsKey, 70594667, 1, null, null, v_userKey, v_userKey, now(), now())
    ;   
END IF;
RETURN;
END;
$$;


CREATE FUNCTION mgd.bib_refs_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: BIB_Refs_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INPUT:
--	none
-- RETURNS:
--	NEW
DELETE FROM IMG_ImagePane_Assoc a
WHERE a._Object_key = OLD._Refs_key
AND a._MGIType_key = 1
;
DELETE FROM MAP_Coord_Feature a
WHERE a._Object_key = OLD._Refs_key
AND a._MGIType_key = 1
;
DELETE FROM MAP_Coordinate a
WHERE a._Object_key = OLD._Refs_key
AND a._MGIType_key = 1
;
DELETE FROM MGI_Note a
WHERE a._Object_key = OLD._Refs_key
AND a._MGIType_key = 1
;
DELETE FROM MGI_Property a
WHERE a._Object_key = OLD._Refs_key
AND a._MGIType_key = 1
;
DELETE FROM MGI_Reference_Assoc a
WHERE a._Object_key = OLD._Refs_key
AND a._MGIType_key = 1
;
DELETE FROM MGI_Synonym a
WHERE a._Object_key = OLD._Refs_key
AND a._MGIType_key = 1
;
DELETE FROM ACC_Accession a
WHERE a._Object_key = OLD._Refs_key
AND a._MGIType_key = 1
;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.bib_refs_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
rec record;
BEGIN
-- NAME: BIB_Refs_insert()
-- DESCRIPTOIN:
--	this insert trigger will call ACC_assignMGI
--	in order to add a distinct MGI accession id
--	to the NEW object
--	adds: J:
--	adds: BIB_Workflow_Status, one row per Group, status = 'New'
--	adds: BIB_Workflow_Data, supplimental term = 'No supplemental data' (34026998)
--	adds: BIB_Workflow_Data, extracted text section = 'body' (48804490)
-- INPUT:
--	none
-- RETURNS:
--	NEW
-- create MGI id
PERFORM ACC_assignMGI(1001, NEW._Refs_key, 'Reference');
-- this is now done in the API where it checks bib_workflow_status
-- create J:
--PERFORM ACC_assignJ(1001, NEW._Refs_key, -1);
FOR rec IN
SELECT _Term_key FROM VOC_Term where _Vocab_key = 127
LOOP
INSERT INTO BIB_Workflow_Status 
VALUES((select nextval('bib_workflow_status_seq')), NEW._Refs_key, rec._Term_key, 71027551, 1,  
	NEW._CreatedBy_key, NEW._ModifiedBy_key, now(), now())
;
END LOOP;
INSERT INTO BIB_Workflow_Data 
VALUES((select nextval('bib_workflow_data_seq')), NEW._Refs_key, 0, 34026998, null, 48804490, null, NEW._CreatedBy_key, NEW._ModifiedBy_key, now(), now())
;
INSERT INTO BIB_Workflow_Relevance
VALUES((select nextval('bib_workflow_relevance_seq')), NEW._Refs_key, 70594667, 1, null, null, NEW._CreatedBy_key, NEW._ModifiedBy_key, now(), now())
;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.bib_reloadcache(v_refskey integer DEFAULT '-1'::integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: BIB_reloadCache
-- DESCRIPTION:
--        
-- To delete/reload the BIB_Citation_Cache
-- 1. by individual reference
-- 
-- 2. if v_refsKey = -1, process all references
-- 
-- 3. if v_refsKey = -2, process references that do not exist in the cache
-- 
-- 4. if needed, add v_refsKey = -3, process references modified during a particular date/time
-- 
-- INPUT:
-- v_refsKey : BIB_Refs._Refs_key
--      
-- RETURNS:
--	VOID
IF (v_refsKey = -1)
THEN
CREATE TEMP TABLE toAdd ON COMMIT DROP
AS SELECT r._Refs_key, null::int as numericPart, null::text as jnumID, a.accID, null::text as pubmedID, null::text as doiID, 
r.journal, 
coalesce(r.journal, '') || ' ' || coalesce(r.date, '') || ';' ||  
coalesce(r.vol, '') || '(' || coalesce(r.issue, '') || '):' ||  
coalesce(r.pgs, '') as citation, 
coalesce(r._primary, '') || ', ' || coalesce(r.journal, '') || ' ' ||  
coalesce(r.date, '') || ';' || coalesce(r.vol, '') || '(' ||  
coalesce(r.issue, '') || '):' || coalesce(r.pgs, '') as short_citation,
s.term as referenceType, 
wr._Relevance_key,
wt.term as relevanceTerm,
r.isReviewArticle, 
case when isReviewArticle = 1 then 'Yes' else 'No' end
FROM BIB_Refs r, VOC_Term s, ACC_Accession a, BIB_Workflow_Relevance wr, VOC_Term wt
WHERE r._ReferenceType_key = s._Term_key 
AND r._Refs_key = a._Object_key 
AND a._MGIType_key = 1 
AND a._LogicalDB_key = 1 
AND a.prefixPart =  'MGI:'
and r._Refs_key = wr._Refs_key
and wr.isCurrent = 1
and wr._Relevance_key = wt._Term_key
;
DELETE FROM BIB_Citation_Cache;
ELSE
CREATE TEMP TABLE toAdd ON COMMIT DROP
AS SELECT r._Refs_key, null::int as numericPart, null::text as jnumID, a.accID, null::text as pubmedID, null::text as doiID, 
r.journal, 
coalesce(r.journal, '') || ' ' || coalesce(r.date, '') || ';' ||  
coalesce(r.vol, '') || '(' || coalesce(r.issue, '') || '):' ||  
coalesce(r.pgs, '') as citation, 
coalesce(r._primary, '') || ', ' || coalesce(r.journal, '') || ' ' ||  
coalesce(r.date, '') || ';' || coalesce(r.vol, '') || '(' ||  
coalesce(r.issue, '') || '):' || coalesce(r.pgs, '') as short_citation,
s.term as referenceType, 
wr._Relevance_key,
wt.term as relevanceTerm,
r.isReviewArticle, 
case when isReviewArticle = 1 then 'Yes' else 'No' end
FROM BIB_Refs r, VOC_Term s, ACC_Accession a, BIB_Workflow_Relevance wr, VOC_Term wt
WHERE r._ReferenceType_key = s._Term_key 
AND r._Refs_key = a._Object_key 
AND a._MGIType_key = 1 
AND a._LogicalDB_key = 1 
AND a.prefixPart =  'MGI:'
AND r._Refs_key = v_refsKey
and r._Refs_key = wr._Refs_key
and wr.isCurrent = 1
and wr._Relevance_key = wt._Term_key
;
DELETE FROM BIB_Citation_Cache WHERE _Refs_key = v_refsKey;
END IF;
CREATE INDEX toAdd_idx1 ON toAdd(_Refs_key);
INSERT INTO BIB_Citation_Cache SELECT * from toAdd
;
UPDATE BIB_Citation_Cache r
SET numericPart = a.numericPart, jnumID = a.accID
FROM toAdd t, ACC_Accession a 
WHERE t._Refs_key = r._Refs_key
AND r._Refs_key = a._Object_key 
AND a._MGIType_key = 1 
AND a._LogicalDB_key = 1 
AND a.prefixPart = 'J:' 
AND a.preferred = 1 
;
UPDATE BIB_Citation_Cache r
SET pubmedID = a.accID
FROM toAdd t, ACC_Accession a 
WHERE t._Refs_key = r._Refs_key
AND r._Refs_key = a._Object_key 
AND a._MGIType_key = 1 
AND a._LogicalDB_key = 29
AND a.preferred = 1 
;
UPDATE BIB_Citation_Cache r
SET doiID = a.accID
FROM toAdd t, ACC_Accession a 
WHERE t._Refs_key = r._Refs_key
AND r._Refs_key = a._Object_key 
AND a._MGIType_key = 1 
AND a._LogicalDB_key = 65
AND a.preferred = 1 
;
DROP TABLE toAdd; 
END;
$$;


CREATE FUNCTION mgd.bib_updatewfstatusap() RETURNS void
    LANGUAGE plpgsql
    AS $$
 
DECLARE
rec record;
BEGIN
-- NAME: BIB_updateWFStatusAP
-- DESCRIPTION:
--        
-- To set the group AP/current Workflow Status = Full-coded
-- where the Reference exists in AP Annotation (_AnnotType_key = 1002)
-- and the group AP/current Workflow Status is *not* Full-coded
-- 
-- To set the group AP/current Workflow Status = Indexed
-- where the Reference exists for an Allele in MGI_Reference_Assoc but there
-- is no MP Annotation snd the group AP/current Workflow Status is *not*
-- Indexed
-- if either: 
-- To set the current Workflow Relevance = keep
-- INPUT:
--	None
-- RETURNS:
--	VOID
--      
FOR rec IN
SELECT r._Refs_key 
FROM BIB_Refs r
WHERE EXISTS (SELECT 1 FROM VOC_Annot a, VOC_Evidence v 
	WHERE r._Refs_key = v._Refs_key
	and v._Annot_key = a._Annot_key
	and a._AnnotType_key = 1002)
AND EXISTS (SELECT 1 FROM BIB_Workflow_Status wfl
        WHERE r._Refs_key = wfl._Refs_key
        AND wfl._Group_key = 31576664
        AND wfl._Status_key != 31576674
        AND wfl.isCurrent = 1)
LOOP
    UPDATE BIB_Workflow_Status w set isCurrent = 0 
    WHERE rec._Refs_key = w._Refs_key
        AND w._Group_key = 31576664
    ;   
    INSERT INTO BIB_Workflow_Status 
    VALUES((select nextval('bib_workflow_status_seq')), rec._Refs_key, 31576664, 31576674, 1, 1001, 1001, now(), now())
    ;   
    PERFORM BIB_keepWFRelevance (rec._Refs_key, 1001);
END LOOP;
FOR rec IN
SELECT distinct r._Refs_key
FROM MGI_Reference_Assoc r
WHERE r._MGIType_key = 11
AND NOT EXISTS (SELECT 1 FROM VOC_Annot a, VOC_Evidence v
        WHERE r._Refs_key = v._Refs_key
        and v._Annot_key = a._Annot_key
        and a._AnnotType_key = 1002)
AND EXISTS (SELECT 1 FROM BIB_Workflow_Status wfl
        WHERE r._Refs_key = wfl._Refs_key
        AND wfl._Group_key = 31576664
        AND wfl._Status_key not in(31576673, 31576674)
        AND wfl.isCurrent = 1)
LOOP
    UPDATE BIB_Workflow_Status w set isCurrent = 0
    WHERE rec._Refs_key = w._Refs_key
        AND w._Group_key = 31576664
    ;
    INSERT INTO BIB_Workflow_Status
    VALUES((select nextval('bib_workflow_status_seq')), rec._Refs_key, 31576664, 31576673, 1, 1001, 1001, now(), now())
    ;
    PERFORM BIB_keepWFRelevance (rec._Refs_key, 1001);
END LOOP;
RETURN;
END;
$$;


CREATE FUNCTION mgd.bib_updatewfstatusgo() RETURNS void
    LANGUAGE plpgsql
    AS $$
 
DECLARE
rec record;
BEGIN
-- NAME: BIB_updateWFStatusGO
-- DESCRIPTION:
--        
-- To set the group GO/current Workflow Status = Full-coded
-- where the Reference exists in GO Annotation (_AnnotType_key = 1000)
-- and the group GO/current Workflow Status is *not* Full-coded
-- INPUT:
--	None
-- RETURNS:
--	VOID
--      
FOR rec IN
SELECT r._Refs_key 
FROM BIB_Refs r
WHERE EXISTS (SELECT 1 FROM VOC_Annot a, VOC_Evidence v 
	WHERE r._Refs_key = v._Refs_key
	and v._Annot_key = a._Annot_key
	and a._AnnotType_key = 1000)
AND EXISTS (SELECT 1 FROM BIB_Workflow_Status wfl
        WHERE r._Refs_key = wfl._Refs_key
        AND wfl._Group_key = 31576666
        AND wfl._Status_key != 31576674
        AND wfl.isCurrent = 1)
LOOP
    UPDATE BIB_Workflow_Status w set isCurrent = 0 
    WHERE rec._Refs_key = w._Refs_key
        AND w._Group_key = 31576666
    ;   
    INSERT INTO BIB_Workflow_Status 
    VALUES((select nextval('bib_workflow_status_seq')), rec._Refs_key, 31576666, 31576674, 1, 1001, 1001, now(), now())
    ;   
    PERFORM BIB_keepWFRelevance (rec._Refs_key, 1001);
END LOOP;
RETURN;
END;
$$;


CREATE FUNCTION mgd.bib_updatewfstatusgxd() RETURNS void
    LANGUAGE plpgsql
    AS $$
 
DECLARE
rec record;
BEGIN
-- NAME: BIB_updateWFStatusGXD
-- DESCRIPTION:
--        
-- To set the group GXD/current Workflow Status = Full-coded
-- where the Reference exists in GXD_Assay
-- and the group GXD/current Workflow Status is *not* Full-coded
-- INPUT:
--	None
-- RETURNS:
--	VOID
--      
FOR rec IN
SELECT r._Refs_key 
FROM BIB_Refs r
WHERE EXISTS (SELECT 1 FROM GXD_Assay a WHERE r._Refs_key = a._Refs_key
	AND a._AssayType_key in (1,2,3,4,5,6,8,9))
AND EXISTS (SELECT 1 FROM BIB_Workflow_Status wfl
        WHERE r._Refs_key = wfl._Refs_key
        AND wfl._Group_key = 31576665
        AND wfl._Status_key != 31576674
        AND wfl.isCurrent = 1)
LOOP
    UPDATE BIB_Workflow_Status w set isCurrent = 0 
    WHERE rec._Refs_key = w._Refs_key
        AND w._Group_key = 31576665
    ;   
    INSERT INTO BIB_Workflow_Status 
    VALUES((select nextval('bib_workflow_status_seq')), rec._Refs_key, 31576665, 31576674, 1, 1001, 1001, now(), now())
    ;   
    PERFORM BIB_keepWFRelevance (rec._Refs_key, 1001);
END LOOP;
RETURN;
END;
$$;


CREATE FUNCTION mgd.bib_updatewfstatusqtl() RETURNS void
    LANGUAGE plpgsql
    AS $$
 
DECLARE
rec record;
BEGIN
-- NAME: BIB_updateWFStatusQTL
-- DESCRIPTION:
--        
-- To set the group QTL/current Workflow Status = Full-coded
-- where the Reference exists in QTL Mapping Experiment
-- and the group QTL/current Workflow Status is *not* Full-coded 
-- INPUT:
--	None
-- RETURNS:
--	VOID
--      
FOR rec IN
SELECT r._Refs_key 
FROM BIB_Refs r
WHERE EXISTS (SELECT 1 from MLD_Expts e
	WHERE r._Refs_key = e._Refs_key
	AND e.expttype in ('TEXT-QTL', 'TEXT-QTL-Candidate Genes', 'TEXT-Congenic', 'TEXT-Meta Analysis'))
AND EXISTS (SELECT 1 FROM BIB_Workflow_Status wfl
        WHERE r._Refs_key = wfl._Refs_key
        AND wfl._Group_key = 31576668
        AND wfl._Status_key not in (31576674)
        AND wfl.isCurrent = 1)
LOOP
    UPDATE BIB_Workflow_Status w set isCurrent = 0 
    WHERE rec._Refs_key = w._Refs_key
        AND w._Group_key = 31576668
    ;   
    INSERT INTO BIB_Workflow_Status 
    VALUES((select nextval('bib_workflow_status_seq')), rec._Refs_key, 31576668, 31576674, 1, 1001, 1001, now(), now())
    ;   
    PERFORM BIB_keepWFRelevance (rec._Refs_key, 1001);
END LOOP;
RETURN;
END;
$$;


CREATE FUNCTION mgd.gxd_addcelltypeset(v_createdby text, v_assaykey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_newSetMemberKey int;
v_newSeqKey int;
v_userKey int;
BEGIN
-- NAME: GXD_addCellTypeSet
-- DESCRIPTION:
-- To add Assay/Cell Types to Cell Ontology set for the clipboard
-- (MGI_SetMember)
--        
-- INPUT:
--      
-- v_createdBy : MGI_User.name
-- v_assayKey : GXD_Assay._Assay_key
-- RETURNS:
--	VOID
-- find existing Cell Ontology terms by Assay
-- add them to MGI_SetMember (clipboard)
-- exclude existing Cell Ontology terms that already exist in the clipboard
-- that is, do not add dupliate structures to the clipboard
v_newSetMemberKey := max(_SetMember_key) FROM MGI_SetMember;
v_userKey = _user_key from MGI_User where login = v_createdBy;
v_newSeqKey := max(sequenceNum) FROM MGI_SetMember where _Set_key = 1059 and _CreatedBy_key = v_userKey;
IF v_newSeqKey is null THEN
	v_newSeqKey := 0;
END IF;
CREATE TEMP TABLE set_tmp ON COMMIT DROP
AS (
SELECT row_number() over (ORDER BY s.term) as seq,
       s._celltype_term_key, s.term, min(s.sequenceNum) as sequenceNum
FROM GXD_ISResultCellType_View s
WHERE s._Assay_key = v_assayKey
AND NOT EXISTS (SELECT 1 FROM MGI_SetMember ms
	WHERE ms._Set_key = 1059
	AND ms._Object_key = s._celltype_term_key
	AND ms._CreatedBy_key = v_userKey
	)
GROUP BY s._celltype_term_key, s.term
)
ORDER BY term
;
INSERT INTO MGI_SetMember
SELECT v_newSetMemberKey + seq, 1059, s._celltype_term_key, s.term, v_newSeqKey + seq, v_userKey, v_userKey, now(), now()
FROM set_tmp s
;
RETURN
;
END;
$$;


CREATE FUNCTION mgd.gxd_addemapaset(v_createdby text, v_assaykey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_newSetMemberKey int;
v_newSetMemberEmapaKey int;
v_newSeqKey int;
v_userKey int;
BEGIN
-- NAME: GXD_addEMAPASet
-- DESCRIPTION:
-- To add Assay/EMAPA/Stage to EMAPA set for the clipboard
-- (MGI_SetMember, MGI_SetMember_EMAPA)
--        
-- INPUT:
--      
-- v_createdBy : MGI_User.name
-- v_assayKey : GXD_Assay._Assay_key
-- RETURNS:
--	VOID
-- find existing EMAPA/Stage terms by Assay
-- add them to MGI_SetMember/MGI_SetMember_EMAPA (clipboard)
-- exclude existing EMAPA/Stages terms that already exist in the clipboard
-- that is, do not add dupliate structures to the clipboard
v_newSetMemberKey := max(_SetMember_key) FROM MGI_SetMember;
v_newSetMemberEmapaKey := max(_SetMember_EMAPA_key) FROM MGI_SetMember_EMAPA;
v_userKey = _user_key from MGI_User where login = v_createdBy;
v_newSeqKey := max(sequenceNum) FROM MGI_SetMember where _Set_key = 1046 and _CreatedBy_key = v_userKey;
IF v_newSetMemberEmapaKey is null THEN
	v_newSetMemberEmapaKey := 0;
END IF;
IF v_newSeqKey is null THEN
	v_newSeqKey := 0;
END IF;
CREATE TEMP TABLE set_tmp ON COMMIT DROP
AS (
SELECT row_number() over (ORDER BY s.stage, s.term) as seq,
       s._EMAPA_Term_key, s._Stage_key, s.term, s.stage, min(s.sequenceNum) as sequenceNum
FROM GXD_GelLaneStructure_View s
WHERE s._Assay_key = v_assayKey
AND NOT EXISTS (SELECT 1 FROM MGI_SetMember ms, MGI_SetMember_EMAPA mse
	WHERE ms._Set_key = 1046
	AND ms._Object_key = s._EMAPA_Term_key
	AND ms._CreatedBy_key = v_userKey
	AND ms._SetMember_key = mse._SetMember_key
	and mse._Stage_key = s._Stage_key)
GROUP BY s._EMAPA_Term_key, s._Stage_key, s.term, s.stage
UNION ALL
SELECT row_number() over (ORDER BY s.stage, s.term) as seq,
       s._EMAPA_Term_key, s._Stage_key, s.term, s.stage, min(ss.sequenceNum) as sequenceNum
FROM GXD_ISResultStructure_View s, GXD_Specimen ss
WHERE ss._Assay_key = v_assayKey
AND ss._Specimen_key = s._Specimen_key 
AND NOT EXISTS (SELECT 1 FROM MGI_SetMember ms, MGI_SetMember_EMAPA mse
	WHERE ms._Set_key = 1046
	AND ms._Object_key = s._EMAPA_Term_key
	AND ms._CreatedBy_key = v_userKey
	AND ms._SetMember_key = mse._SetMember_key
	and mse._Stage_key = s._Stage_key)
GROUP BY s._EMAPA_Term_key, s._Stage_key, s.term, s.stage
)
ORDER BY stage, term
;
INSERT INTO MGI_SetMember
SELECT v_newSetMemberKey + seq, 1046, s._EMAPA_Term_key, null, v_newSeqKey + seq, v_userKey, v_userKey, now(), now()
FROM set_tmp s
;
INSERT INTO MGI_SetMember_EMAPA
SELECT v_newSetMemberEmapaKey + seq, v_newSetMemberKey + seq, s._Stage_key, v_userKey, v_userKey, now(), now()
FROM set_tmp s
;
RETURN
;
END;
$$;


CREATE FUNCTION mgd.gxd_addgenotypeset(v_createdby text, v_assaykey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_newSetMemberKey int;
v_newSeqKey int;
v_userKey int;
BEGIN
-- NAME: GXD_addGenotypeSet
-- DESCRIPTION:
-- To add Assay/Genotypes to Genotype set for the clipboard
-- (MGI_SetMember)
--        
-- INPUT:
--      
-- v_createdBy : MGI_User.name
-- v_assayKey : GXD_Assay._Assay_key
-- RETURNS:
--	VOID
-- find existing Genotype by Assay
-- add them to MGI_SetMember (clipboard)
-- exclude existing Genotype that already exist in the clipboard
-- that is, do not add dupliate structures to the clipboard
v_newSetMemberKey := max(_SetMember_key) FROM MGI_SetMember;
v_userKey = _user_key from MGI_User where login = v_createdBy;
v_newSeqKey := max(sequenceNum) FROM MGI_SetMember where _Set_key = 1055 and _CreatedBy_key = v_userKey;
IF v_newSeqKey is null THEN
	v_newSeqKey := 0;
END IF;
CREATE TEMP TABLE set_tmp ON COMMIT DROP
AS (
SELECT row_number() over (ORDER BY ps.strain) as seq,
        g._genotype_key, ps.strain, concat(ps.strain,',',a0.symbol,',',aa0.symbol) as genotypeDisplay
FROM GXD_Assay ga, GXD_Specimen s, GXD_Genotype g
        LEFT OUTER JOIN prb_strain ps on (g._strain_key = ps._strain_key)
        LEFT OUTER JOIN gxd_allelepair ap0 on (g._genotype_key = ap0._genotype_key)
        LEFT OUTER JOIN all_allele a0 on (ap0._allele_key_1 = a0._allele_key)
        LEFT OUTER JOIN all_allele aa0 on (ap0._allele_key_2 = aa0._allele_key)
, MRK_Marker m0
where ga._assay_key = v_assayKey
and ga._assay_key = s._assay_key
and s._genotype_key = g._genotype_key
and ga._marker_key = m0._marker_key
AND NOT EXISTS (SELECT 1 FROM MGI_SetMember ms
	WHERE ms._Set_key = 1055
	AND ms._Object_key = s._genotype_key
	AND ms._CreatedBy_key = v_userKey
	)
GROUP BY g._genotype_key, ps.strain, genotypeDisplay
UNION
SELECT row_number() over (ORDER BY ps.strain) as seq,
        g._genotype_key, ps.strain, concat(ps.strain,',',a0.symbol,',',aa0.symbol) as genotypeDisplay
FROM GXD_Assay ga, GXD_GelLane s, GXD_Genotype g
        LEFT OUTER JOIN prb_strain ps on (g._strain_key = ps._strain_key)
        LEFT OUTER JOIN gxd_allelepair ap0 on (g._genotype_key = ap0._genotype_key)
        LEFT OUTER JOIN all_allele a0 on (ap0._allele_key_1 = a0._allele_key)
        LEFT OUTER JOIN all_allele aa0 on (ap0._allele_key_2 = aa0._allele_key)
, MRK_Marker m0
where ga._assay_key = v_assayKey
and ga._assay_key = s._assay_key
and s._genotype_key = g._genotype_key
and ga._marker_key = m0._marker_key
AND NOT EXISTS (SELECT 1 FROM MGI_SetMember ms
	WHERE ms._Set_key = 1055
	AND ms._Object_key = s._genotype_key
	AND ms._CreatedBy_key = v_userKey
	)
GROUP BY g._genotype_key, ps.strain, genotypeDisplay
)
ORDER BY genotypeDisplay
;
INSERT INTO MGI_SetMember
SELECT v_newSetMemberKey + seq, 1055, s._genotype_key, genotypeDisplay, v_newSeqKey + seq, v_userKey, v_userKey, now(), now()
FROM set_tmp s
;
RETURN
;
END;
$$;


CREATE FUNCTION mgd.gxd_allelepair_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: GXD_AllelePair_insert()
-- DESCRIPTOIN:
-- INPUT:
--	none
-- RETURNS:
--	NEW
-- Pair State error checks
IF (select t.term from VOC_Term t where NEW._PairState_key = t._Term_key) = 'Homozygous'
   AND
   (NEW._Allele_key_1 != NEW._Allele_key_2)
THEN
	RAISE EXCEPTION E'State Attribute Error:  Homozygous state is not valid.';
ELSIF (select t.term from VOC_Term t where NEW._PairState_key = t._Term_key) = 'Heterozygous'
   and not
   ( 
     NEW._Allele_key_1 != NEW._Allele_key_2
     and
     NEW._Allele_key_2 is not null
   )
THEN
	RAISE EXCEPTION E'State Attribute Error:  Heterozygous state is not valid.';
ELSIF NEW._Marker_key is not null
   and (select t.term from VOC_Term t where NEW._PairState_key = t._Term_key) = 'Hemizygous X-linked'
   and not
   (
     NEW._Allele_key_2 is null
     and
     (select t.term from VOC_Term t where NEW._Compound_key = t._Term_key) = 'Not Applicable'
     and
     ((select m.chromosome from MRK_Marker m where NEW._Marker_key = m._Marker_key) = 'X' or NEW._Marker_key is null)
   )
THEN
	RAISE EXCEPTION E'State Attribute Error:  Hemizygous X-linked state is not valid.';
ELSIF NEW._Marker_key is not null
   and (select t.term from VOC_Term t where NEW._PairState_key = t._Term_key) = 'Hemizygous Y-linked'
   and not
   (
     (NEW._Allele_key_2 is null
     and
     (select t.term from VOC_Term t where NEW._Compound_key = t._Term_key) = 'Not Applicable'
     and
     ( (select m.chromosome from MRK_Marker m where NEW._Marker_key = m._Marker_key) = 'Y') 
		or NEW._Marker_key is null)
   )
THEN
	RAISE EXCEPTION E'State Attribute Error:  Hemizygous Y-linked state is not valid.';
ELSIF (select t.term from VOC_Term t where NEW._PairState_key = t._Term_key) = 'Hemizygous Insertion'
   and not ( NEW._Allele_key_2 is null)
THEN
	RAISE EXCEPTION E'State Attribute Error:  Hemizygous Insertion state is not valid.';
ELSIF (select t.term from VOC_Term t where NEW._PairState_key = t._Term_key) = 'Hemizygous Deletion'
   and not
   (
     NEW._Allele_key_2 is null
     and
     (select t.term from VOC_Term t where NEW._Compound_key = t._Term_key) != 'Not Applicable'
   )
THEN
	RAISE EXCEPTION E'State Attribute Error:  Hemizygous Deletion state is not valid.';
ELSIF (select t.term from VOC_Term t where NEW._PairState_key = t._Term_key) = 'Indeterminate'
   and not
   (
     NEW._Allele_key_2 is null
     and
     (select t.term from VOC_Term t where NEW._Compound_key = t._Term_key) = 'Not Applicable'
   )
THEN
	RAISE EXCEPTION E'State Attribute Error:  Indeterminate state is not valid.';
END IF;
-- Compound error checks
if (select t.term from VOC_Term t where NEW._Compound_key = t._Term_key) != 'Not Applicable'
THEN
   IF not ( NEW._Allele_key_2 is null
            or
            (select a.isWildType from ALL_Allele a where NEW._Allele_key_2 = a._Allele_key) = 1
          )
   THEN
	RAISE EXCEPTION E'Compound Attribute Error:  Allele 2 for all Compound Allele Pairs must be null or wild type.';
   END IF;
   IF (select t.term from VOC_Term t where NEW._PairState_key = t._Term_key) 
	in ('Hemizygous X-linked', 'Hemizygous Y-linked', 'Indeterminate')
   THEN
	RAISE EXCEPTION E'Allele Compound Error:  Top/Bottom compound is not valid for Hemi-X, Hemi-Y or Indeterminate states.';
   END IF;
END IF;
/* Mutant Cell Line error checks */
IF NEW._MutantCellLine_key_1 is not null
   and not exists (select 1 from ALL_Allele_CellLine a
	where NEW._Allele_key_1 = a._Allele_key
	and NEW._MutantCellLine_key_1 = a._MutantCellLine_key)
THEN
     RAISE EXCEPTION E'Allele 1 and Mutant Cell Line 1 are not compatable.';
END IF;
IF NEW._MutantCellLine_key_2 is not null
   and not exists (select 1 from ALL_Allele_CellLine a
	where NEW._Allele_key_2 = a._Allele_key
	and NEW._MutantCellLine_key_2 = a._MutantCellLine_key)
THEN
     RAISE EXCEPTION E'Allele 2 and Mutant Cell Line 2 are not compatable.';
END IF;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.gxd_antibody_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: GXD_Antibody_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INPUT:
--	none
-- RETURNS:
--	NEW
-- delete GXD_AntibodyPrep records if they are no longer used in Assays
-- then the system should allow the probe to be deleted
DELETE FROM GXD_AntibodyPrep p
WHERE p._Antibody_key = OLD._Antibody_key
AND NOT EXISTS (SELECT 1 FROM GXD_Assay a WHERE p._AntibodyPrep_key = a._AntibodyPrep_key)
;
DELETE FROM IMG_ImagePane_Assoc a
WHERE a._Object_key = OLD._Antibody_key
AND a._MGIType_key = 6
;
DELETE FROM MAP_Coord_Feature a
WHERE a._Object_key = OLD._Antibody_key
AND a._MGIType_key = 6
;
DELETE FROM MAP_Coordinate a
WHERE a._Object_key = OLD._Antibody_key
AND a._MGIType_key = 6
;
DELETE FROM MGI_Note a
WHERE a._Object_key = OLD._Antibody_key
AND a._MGIType_key = 6
;
DELETE FROM MGI_Property a
WHERE a._Object_key = OLD._Antibody_key
AND a._MGIType_key = 6
;
DELETE FROM MGI_Reference_Assoc a
WHERE a._Object_key = OLD._Antibody_key
AND a._MGIType_key = 6
;
DELETE FROM MGI_Synonym a
WHERE a._Object_key = OLD._Antibody_key
AND a._MGIType_key = 6
;
DELETE FROM ACC_Accession a
WHERE a._Object_key = OLD._Antibody_key
AND a._MGIType_key = 6
;
RETURN OLD;
END;
$$;


CREATE FUNCTION mgd.gxd_antibody_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: GXD_Antibody_insert()
-- DESCRIPTOIN:
--	this insert trigger will call ACC_assignMGI
--	in order to add a distinct MGI accession id
--	to the NEW object
-- INPUT:
--	none
-- RETURNS:
--	NEW
PERFORM ACC_assignMGI(1001, NEW._Antibody_key, 'Antibody');
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.gxd_antigen_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: GXD_Antigen_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INPUT:
--	none
-- RETURNS:
--	NEW
-- Antigens have uniq sources, delete the source when the antigen is deleted
DELETE FROM PRB_SOURCE a
WHERE a._Source_key = OLD._Source_key
;
DELETE FROM IMG_ImagePane_Assoc a
WHERE a._Object_key = OLD._Antigen_key
AND a._MGIType_key = 7
;
DELETE FROM MAP_Coord_Feature a
WHERE a._Object_key = OLD._Antigen_key
AND a._MGIType_key = 7
;
DELETE FROM MAP_Coordinate a
WHERE a._Object_key = OLD._Antigen_key
AND a._MGIType_key = 7
;
DELETE FROM MGI_Note a
WHERE a._Object_key = OLD._Antigen_key
AND a._MGIType_key = 7
;
DELETE FROM MGI_Property a
WHERE a._Object_key = OLD._Antigen_key
AND a._MGIType_key = 7
;
DELETE FROM MGI_Reference_Assoc a
WHERE a._Object_key = OLD._Antigen_key
AND a._MGIType_key = 7
;
DELETE FROM MGI_Synonym a
WHERE a._Object_key = OLD._Antigen_key
AND a._MGIType_key = 7
;
DELETE FROM ACC_Accession a
WHERE a._Object_key = OLD._Antigen_key
AND a._MGIType_key = 7
;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.gxd_antigen_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: GXD_Antigen_insert()
-- DESCRIPTOIN:
--	this insert trigger will call ACC_assignMGI
--	in order to add a distinct MGI accession id
--	to the NEW object
-- INPUT:
--	none
-- RETURNS:
--	NEW
PERFORM ACC_assignMGI(1001, NEW._Antigen_key, 'Antigen');
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.gxd_assay_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: GXD_Assay_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INPUT:
--	none
-- RETURNS:
--	NEW
DELETE FROM IMG_ImagePane_Assoc a
WHERE a._Object_key = OLD._Assay_key
AND a._MGIType_key = 8
;
DELETE FROM MAP_Coord_Feature a
WHERE a._Object_key = OLD._Assay_key
AND a._MGIType_key = 8
;
DELETE FROM MAP_Coordinate a
WHERE a._Object_key = OLD._Assay_key
AND a._MGIType_key = 8
;
DELETE FROM MGI_Note a
WHERE a._Object_key = OLD._Assay_key
AND a._MGIType_key = 8
;
DELETE FROM MGI_Property a
WHERE a._Object_key = OLD._Assay_key
AND a._MGIType_key = 8
;
DELETE FROM MGI_Reference_Assoc a
WHERE a._Object_key = OLD._Assay_key
AND a._MGIType_key = 8
;
DELETE FROM MGI_Synonym a
WHERE a._Object_key = OLD._Assay_key
AND a._MGIType_key = 8
;
DELETE FROM ACC_Accession a
WHERE a._Object_key = OLD._Assay_key
AND a._MGIType_key = 8
;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.gxd_assay_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
v_assocKey int;
BEGIN
-- NAME: GXD_Assay_insert()
-- DESCRIPTOIN:
--	this insert trigger will call ACC_assignMGI
--	in order to add a distinct MGI accession id
--	to the NEW object
--	1) if the Reference exists as group = 'Expression'
--		in any BIB_Workflow_Status,
--              and BIB_Workflow_Status is not equal to "Full-coded"
--              and BIB_Workflow_Status.isCurrent = 1
--	   then
--		set existing BIB_Workflow_Status.isCurrent = 0
--		add new BIB_Workflow_Status._Status_key = 'Full-coded'
-- changes to this trigger require changes to procedure/BIB_updateWFStatusQTL_create.object
-- INPUT:
--	none
-- RETURNS:
--	NEW
IF EXISTS (SELECT 1 FROM BIB_Workflow_Status b
        WHERE NEW._Refs_key = b._Refs_key
        AND b._Group_key = 31576665
        AND b._Status_key != 31576674
	AND b.isCurrent = 1)
   AND NEW._AssayType_key in (1,2,3,4,5,6,8,9)
THEN
    UPDATE BIB_Workflow_Status w set isCurrent = 0
    WHERE NEW._Refs_key = w._Refs_key
        AND w._Group_key = 31576665
    ;
    INSERT INTO BIB_Workflow_Status 
    VALUES((select nextval('bib_workflow_status_seq')), NEW._Refs_key, 31576665, 31576674, 1, 
    	NEW._CreatedBy_key, NEW._ModifiedBy_key, now(), now())
    ;
    PERFORM BIB_keepWFRelevance (NEW._Refs_key, NEW._CreatedBy_key);
END IF;
PERFORM ACC_assignMGI(1001, NEW._Assay_key, 'Assay');
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.gxd_checkduplicategenotype(v_genotypekey bigint) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_dupgenotypeKey int;
v_isDuplicate int;
BEGIN
-- NAME: GXD_checkDuplicateGenotype
-- DESCRIPTION:
--        
-- To check if Genotype is a duplicate
-- INPUT:
--      
-- v_genotypeKey : GXD_Genotype._Genotype_key
-- RETURNS:
--	always VOID
--	but RAISE EXCPEPTION if duplicate Genotype is found
--      
-- Check if the given genotype record is a duplicate.
-- If it is a duplicate, the transaction is rolled back. 
-- SELECT the set of all genotypes with the same 
-- 	strain                                   
-- 	conditional flag                         
-- 	allele 1                                 
-- 	pair state                               
-- as the given genotype                         
--                                               
-- allele 2, mcl 1, mcl 2 can all be null        
-- so need to check this in the for-each loop    
-- (exclude the given genotype FROM this set)    
--                                                         
-- for each genotype that may be a duplicate...            
--                                                         
--	compare the set of Allele Pairs                    
--		of the given genotype                      
-- 	to the set of Allele Pairs of the other genotype   
--                                                         
-- 	if the sets are exactly equal,                     
--             then the given genotype is a duplicate      
--                                                         
FOR v_dupgenotypeKey IN
SELECT DISTINCT g1._Genotype_key
FROM GXD_Genotype g1, GXD_AllelePair ap1, GXD_Genotype g2, GXD_AllelePair ap2
WHERE g2._Genotype_key = v_genotypeKey
AND g1._Strain_key = g2._Strain_key
AND g1.isConditional = g2.isConditional
AND g1._Genotype_key != v_genotypeKey
AND g1._Genotype_key = ap1._Genotype_key
AND g2._Genotype_key = ap2._Genotype_key
AND ap1._Marker_key = ap2._Marker_key
AND ap1._Allele_key_1 = ap2._Allele_key_1
AND ap1._PairState_key = ap2._PairState_key
UNION
SELECT DISTINCT g1._Genotype_key
FROM GXD_Genotype g1, GXD_Genotype g2
WHERE g2._Genotype_key = v_genotypeKey
AND g1._Strain_key = g2._Strain_key
AND g1.isConditional = g2.isConditional
AND g1._Genotype_key != v_genotypeKey
AND NOT exists (SELECT 1 FROM GXD_AllelePair ap1 WHERE g1._Genotype_key = ap1._Genotype_key)
AND NOT exists (SELECT 1 FROM GXD_AllelePair ap2 WHERE g2._Genotype_key = ap2._Genotype_key)
LOOP
	-- if the # of Allele Pairs is different, then it's not a duplicate
	IF (SELECT count(*) FROM GXD_AllelePair WHERE _Genotype_key = v_genotypeKey) != 
	   (SELECT count(*) FROM GXD_AllelePair WHERE _Genotype_key = v_dupgenotypeKey)
	THEN
		v_isDuplicate := 0;
	ELSIF EXISTS (SELECT 1 FROM GXD_AllelePair g1
		WHERE g1._Genotype_key = v_genotypeKey
		AND g1._Allele_key_2 is not null
		AND g1._MutantCellLine_key_1 is not null
		AND g1._MutantCellLine_key_2 is not null
		AND NOT EXISTS (SELECT 1 FROM GXD_AllelePair g2
			WHERE g2._Genotype_key = v_dupgenotypeKey
			AND g1._Allele_key_1 = g2._Allele_key_1
			AND g1._Allele_key_2 = g2._Allele_key_2
			AND g1._MutantCellLine_key_1 = g2._MutantCellLine_key_1
			AND g1._MutantCellLine_key_2 = g2._MutantCellLine_key_2
			AND g1._PairState_key = g2._PairState_key)) THEN
		v_isDuplicate := 0;
	
	ELSIF EXISTS (SELECT 1 FROM GXD_AllelePair g1
		WHERE g1._Genotype_key = v_genotypeKey
		AND g1._Allele_key_2 is not null
		AND g1._MutantCellLine_key_1 IS NULL
		AND g1._MutantCellLine_key_2 is not null
		AND NOT EXISTS (SELECT 1 FROM GXD_AllelePair g2
			WHERE g2._Genotype_key = v_dupgenotypeKey
			AND g1._Allele_key_1 = g2._Allele_key_1
			AND g1._Allele_key_2 = g2._Allele_key_2
			AND g2._MutantCellLine_key_1 IS NULL
			AND g1._MutantCellLine_key_2 = g2._MutantCellLine_key_2
			AND g1._PairState_key = g2._PairState_key)) THEN
		v_isDuplicate := 0;
	
	ELSIF EXISTS (SELECT 1 FROM GXD_AllelePair g1
		WHERE g1._Genotype_key = v_genotypeKey
		AND g1._Allele_key_2 is not null
		AND g1._MutantCellLine_key_1 is not null
		AND g1._MutantCellLine_key_2 IS NULL
		AND NOT EXISTS (SELECT 1 FROM GXD_AllelePair g2
			WHERE g2._Genotype_key = v_dupgenotypeKey
			AND g1._Allele_key_1 = g2._Allele_key_1
			AND g1._Allele_key_2 = g2._Allele_key_2
			AND g1._MutantCellLine_key_1 = g2._MutantCellLine_key_1
			AND g2._MutantCellLine_key_2 IS NULL
			AND g1._PairState_key = g2._PairState_key)) THEN
		v_isDuplicate := 0;
	
	ELSIF EXISTS (SELECT 1 FROM GXD_AllelePair g1
		WHERE g1._Genotype_key = v_genotypeKey
		AND g1._Allele_key_2 is not null
		AND g1._MutantCellLine_key_1 IS NULL
		AND g1._MutantCellLine_key_2 IS NULL
		AND NOT EXISTS (SELECT 1 FROM GXD_AllelePair g2
			WHERE g2._Genotype_key = v_dupgenotypeKey
			AND g1._Allele_key_1 = g2._Allele_key_1
			AND g1._Allele_key_2 = g2._Allele_key_2
			AND g2._MutantCellLine_key_1 IS NULL
			AND g2._MutantCellLine_key_2 IS NULL
			AND g1._PairState_key = g2._PairState_key)) THEN
		v_isDuplicate := 0;
	
	ELSIF EXISTS (SELECT 1 FROM GXD_AllelePair g1
		WHERE g1._Genotype_key = v_genotypeKey
		AND g1._Allele_key_2 IS NULL
		AND g1._MutantCellLine_key_1 is not null
		AND g1._MutantCellLine_key_2 IS NULL
		AND NOT EXISTS (SELECT 1 FROM GXD_AllelePair g2
			WHERE g2._Genotype_key = v_dupgenotypeKey
			AND g1._Allele_key_1 = g2._Allele_key_1
			AND g2._Allele_key_2 IS NULL
			AND g1._MutantCellLine_key_1 = g2._MutantCellLine_key_1
			AND g2._MutantCellLine_key_2 IS NULL
			AND g1._PairState_key = g2._PairState_key)) THEN
		v_isDuplicate := 0;
	
	ELSIF EXISTS (SELECT 1 FROM GXD_AllelePair g1
		WHERE g1._Genotype_key = v_genotypeKey
		AND g1._Allele_key_2 IS NULL
		AND g1._MutantCellLine_key_1 IS NULL
		AND g1._MutantCellLine_key_2 IS NULL
		AND NOT EXISTS (SELECT 1 FROM GXD_AllelePair g2
			WHERE g2._Genotype_key = v_dupgenotypeKey
			AND g1._Allele_key_1 = g2._Allele_key_1
			AND g2._Allele_key_2 IS NULL
			AND g2._MutantCellLine_key_1 IS NULL
			AND g2._MutantCellLine_key_2 IS NULL
			AND g1._PairState_key = g2._PairState_key)) THEN
		v_isDuplicate := 0;
	
	ELSE
		v_isDuplicate := 1;
		EXIT;
	END IF;
END LOOP;
IF v_isDuplicate = 1
THEN
        RAISE EXCEPTION E'Duplicate Genotype Detected.';
        RETURN;
END IF;
END;
$$;


CREATE FUNCTION mgd.gxd_gelrow_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: GXD_GelRow_insert()
-- DESCRIPTOIN:
--	
-- 1) If Gel Row Size is entered, Gel Row Units must be specified.
-- INPUT:
--	none
-- RETURNS:
--	NEW
IF NEW.size IS NOT NULL AND NEW._GelUnits_key < 0
THEN
  RAISE EXCEPTION E'If Gel Row Size is entered, Gel Row Units must be specified.';
END IF;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.gxd_genotype_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: GXD_Genotype_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INPUT:
--	none
-- RETURNS:
--	NEW
IF EXISTS (SELECT * FROM VOC_Annot a
          WHERE OLD._Genotype_key = a._Object_key
	  AND a._AnnotType_key = 1002)
THEN
        RAISE EXCEPTION E'Genotype is referenced in Mammalian Phenotype/Genotype Annotations';
END IF; 
IF EXISTS (SELECT * FROM VOC_Annot a
          WHERE OLD._Genotype_key = a._Object_key
	  AND a._AnnotType_key = 1020)
THEN
        RAISE EXCEPTION E'Genotype is referenced in DO/Genotype Annotations';
END IF; 
DELETE FROM IMG_ImagePane_Assoc a
WHERE a._Object_key = OLD._Genotype_key
AND a._MGIType_key = 12
;
DELETE FROM MAP_Coord_Feature a
WHERE a._Object_key = OLD._Genotype_key
AND a._MGIType_key = 12
;
DELETE FROM MAP_Coordinate a
WHERE a._Object_key = OLD._Genotype_key
AND a._MGIType_key = 12
;
DELETE FROM MGI_Note a
WHERE a._Object_key = OLD._Genotype_key
AND a._MGIType_key = 12
;
DELETE FROM MGI_Property a
WHERE a._Object_key = OLD._Genotype_key
AND a._MGIType_key = 12
;
DELETE FROM MGI_Reference_Assoc a
WHERE a._Object_key = OLD._Genotype_key
AND a._MGIType_key = 12
;
DELETE FROM MGI_Synonym a
WHERE a._Object_key = OLD._Genotype_key
AND a._MGIType_key = 12
;
DELETE FROM VOC_AnnotHeader a
WHERE a._Object_key = OLD._Genotype_key
AND a._AnnotType_key = 1002
;
DELETE FROM ACC_Accession a
WHERE a._Object_key = OLD._Genotype_key
AND a._MGIType_key = 12
;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.gxd_genotype_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: GXD_Genotype_insert()
-- DESCRIPTOIN:
--	this insert trigger will call ACC_assignMGI
--	in order to add a distinct MGI accession id
--	to the NEW object
-- INPUT:
--	none
-- RETURNS:
--	NEW
PERFORM ACC_assignMGI(1001, NEW._Genotype_key, 'Genotype');
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.gxd_getgenotypesdatasets(v_genotypekey integer) RETURNS TABLE(_refs_key integer, jnum integer, jnumid text, short_citation text, dataset text)
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: GXD_getGenotypesDataSets
-- DESCRIPTION:
--        
-- To select all references/data sets that a given Genotype is associated with
-- INPUT:
-- v_genotypeKey : GXD_Genotyype._Genotype_key
--      
-- RETURNS:
--	TABLE
--	_refs_key
--	jnum
--	jnumid
--	short_citation
--	dataSet
--      
-- Select all Data Sets for given Genotype 
CREATE TEMP TABLE objects ON COMMIT DROP
AS SELECT DISTINCT s._Refs_key, 'Reference' as label, 'GXD Assay' as dataSet
FROM GXD_Expression s
WHERE s._Genotype_key = v_genotypeKey
UNION ALL
SELECT DISTINCT e._Refs_key, 'Reference' as label, 'PhenoSlim Annotation' as dataSet
FROM VOC_Evidence e, VOC_Annot a, VOC_AnnotType t
WHERE t.name = 'PhenoSlim/Genotype'
AND t._AnnotType_key = a._AnnotType_key
AND a._Object_key = v_genotypeKey
AND a._Annot_key = e._Annot_key
UNION ALL
SELECT DISTINCT e._Refs_key, 'Reference' as label, 'Mammalian Phenotype Annotation' as dataSet
FROM VOC_Evidence e, VOC_Annot a, VOC_AnnotType t
WHERE t.name = 'Mammalian Phenotype/Genotype'
AND t._AnnotType_key = a._AnnotType_key
AND a._Object_key = v_genotypeKey
AND a._Annot_key = e._Annot_key
UNION ALL
SELECT DISTINCT e._Refs_key, 'Reference' as label, 'DO Annotation' as dataSet
FROM VOC_Evidence e, VOC_Annot a
where a._AnnotType_key = 1020
AND a._Object_key = v_genotypeKey
AND a._Annot_key = e._Annot_key
UNION ALL
SELECT DISTINCT e._Refs_key, 'Reference' as label, 'DO Annotation' as dataSet
FROM VOC_Evidence e, VOC_Annot a
where a._AnnotType_key = 1020
AND a._Object_key = v_genotypeKey
AND a._Annot_key = e._Annot_key
UNION ALL
SELECT DISTINCT 0 as _Refs_key, s.strain as label, 'Strain' as dataSet
FROM PRB_Strain_Genotype ps, PRB_Strain s
WHERE ps._Genotype_key = v_genotypeKey
AND ps._Strain_key = s._Strain_key
UNION ALL
SELECT DISTINCT 0 as _Refs_key, a.name as label, 'HT Samples' as dataSet
FROM GXD_HTSample a
where a._Genotype_key = v_genotypeKey
;
CREATE INDEX idx1 ON objects(_Refs_key);
RETURN QUERY
SELECT b._refs_key, b.jnum, b.jnumid, b.short_citation, o.dataSet
FROM objects o, BIB_View b
WHERE o._Refs_key = b._Refs_key
UNION
SELECT NULL, NULL, NULL, o.label, o.dataSet
FROM objects o
WHERE o._Refs_key = 0
ORDER BY jnum
;
END;
$$;


CREATE FUNCTION mgd.gxd_getgenotypesdatasetscount(v_genotypekey integer) RETURNS integer
    LANGUAGE plpgsql
    AS $$
BEGIN
-- Select all Data Sets for given Genotype 
CREATE TEMP TABLE objects ON COMMIT DROP
AS SELECT DISTINCT s._Refs_key, 'Reference' as label, 'GXD Assay' as dataSet
FROM GXD_Expression s
WHERE s._Genotype_key = v_genotypeKey
UNION
SELECT DISTINCT e._Refs_key, 'Reference' as label, 'Mammalian Phenotype Annotation' as dataSet
FROM VOC_Evidence e, VOC_Annot a
WHERE a._AnnotType_key = 1002
AND a._Object_key = v_genotypeKey
AND a._Annot_key = e._Annot_key
UNION
SELECT DISTINCT e._Refs_key, 'Reference' as label, 'DO Annotation' as dataSet
FROM VOC_Evidence e, VOC_Annot a
where a._AnnotType_key = 1020
AND a._Object_key = v_genotypeKey
AND a._Annot_key = e._Annot_key
UNION
SELECT DISTINCT 0 as _Refs_key, s.strain as label, 'Strain' as dataSet
FROM PRB_Strain_Genotype ps, PRB_Strain s
WHERE ps._Genotype_key = v_genotypeKey
AND ps._Strain_key = s._Strain_key
UNION
SELECT DISTINCT 0 as _Refs_key, a.name as label, 'HT Sample' as dataSet
FROM GXD_HTSample a
WHERE a._Genotype_key = v_genotypeKey
;
CREATE INDEX idx1 ON objects(_Refs_key);
RETURN (SELECT count(*) from objects);
END;
$$;


CREATE FUNCTION mgd.gxd_htexperiment_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: GXD_HTExperiment_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INPUT:
--	none
-- RETURNS:
--	NEW
DELETE FROM MGI_Property a
WHERE a._Object_key = OLD._Experiment_key
AND a._MGIType_key = 42
;
DELETE FROM ACC_Accession a
WHERE a._Object_key = OLD._Experiment_key
AND a._MGIType_key = 42
;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.gxd_htrawsample_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: GXD_HTRawSample_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INPUT:
--	none
-- RETURNS:
--	NEW
DELETE FROM MGI_KeyValue a
WHERE a._Object_key = OLD._RawSample_key
AND a._MGIType_key = 47
;
DELETE FROM ACC_Accession a
WHERE a._Object_key = OLD._RawSample_key
AND a._MGIType_key = 47
;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.gxd_htsample_ageminmax() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: GXD_HTSample_ageminmax()
-- DESCRIPTOIN:
--	
-- After insert or update of GXD_HTSample, set the ageMin/ageMax
-- INPUT:
--	none
-- RETURNS:
--	NEW
IF (TG_OP = 'UPDATE') THEN
	PERFORM MGI_resetAgeMinMax ('GXD_HTSample', OLD._Sample_key);
ELSE
	PERFORM MGI_resetAgeMinMax ('GXD_HTSample', NEW._Sample_key);
END IF;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.gxd_index_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
v_assocKey int;
BEGIN
-- NAME: GXD_Index_insert()
-- DESCRIPTOIN:
--	1) if the Reference exists as group = 'Expression'
--		and status not in ('Full-coded')
--		in BIB_Workflow_Status,
--	   then
--		set existing BIB_Workflow_Status.isCurrent = 0
--		add new BIB_Workflow_Status._Status_key = 'Indexed'
-- changes to this trigger require changes to procedure/BIB_updateWFStatusGXD_create.object
-- INPUT:
--	none
-- RETURNS:
--	NEW
IF EXISTS (SELECT 1 FROM BIB_Workflow_Status b
        WHERE NEW._Refs_key = b._Refs_key
        AND b._Group_key = 31576665
	AND b._Status_key not in (31576674))
THEN
    UPDATE BIB_Workflow_Status w set isCurrent = 0
    WHERE NEW._Refs_key = w._Refs_key
        AND w._Group_key = 31576665
    ;
    INSERT INTO BIB_Workflow_Status 
    VALUES((select nextval('bib_workflow_status_seq')), NEW._Refs_key, 31576665, 31576673, 1, 
    	NEW._CreatedBy_key, NEW._ModifiedBy_key, now(), now())
    ;
    PERFORM BIB_keepWFRelevance (NEW._Refs_key, NEW._CreatedBy_key);
END IF;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.gxd_index_insert_before() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: GXD_Index_insert_before()
-- DESCRIPTOIN:
--	set the _Priority_key/_ConditionalMutants_key values as necessary
--	BEFORE insert
-- INPUT:
--	none
-- RETURNS:
--	NEW
-- if reference is not currently used and priority is null, no default
IF NOT EXISTS (SELECT 1 FROM GXD_Index WHERE _Refs_key = NEW._Refs_key)
   AND NEW._Priority_key IS NULL
THEN
    RAISE EXCEPTION E'Priority Required';
END IF;
-- if reference is not currently used and conditional is null, default is 'Not Applicable'
IF NOT EXISTS (SELECT 1 FROM GXD_Index WHERE _Refs_key = NEW._Refs_key)
   AND NEW._ConditionalMutants_key IS NULL
THEN
   NEW._ConditionalMutants_key := 4834242;
END IF;
-- if reference is used...
IF EXISTS (SELECT 1 FROM GXD_Index WHERE _Refs_key = NEW._Refs_key)
THEN
    -- if priority is null, then use default
    IF NEW._Priority_key IS NULL
    THEN
        NEW._Priority_key := DISTINCT _Priority_key FROM GXD_Index WHERE _Refs_key = NEW._Refs_key;
    END IF;
    -- if conditional is null, then use default
    IF NEW._ConditionalMutants_key IS NULL
    THEN
        NEW._ConditionalMutants_key := DISTINCT _ConditionalMutants_key FROM GXD_Index WHERE _Refs_key = NEW._Refs_key;
    END IF;
    -- use priority/conditional for every instance of the reference
    UPDATE GXD_Index
    SET _Priority_key = NEW._Priority_key,
        _ConditionalMutants_key = NEW._ConditionalMutants_key 
    WHERE _Refs_key = NEW._Refs_key
    ;
END IF;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.gxd_index_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: GXD_Index_update()
-- DESCRIPTOIN:
--	update all _Priority_key/_ConditionalMutants_key for all instances of NEW._Refs_key
--	only if index count < 2000
-- INPUT:
--	none
-- RETURNS:
--	NEW
IF (SELECT COUNT(*) FROM GXD_Index WHERE _Refs_key = NEW._Refs_key) < 2000
THEN
	UPDATE GXD_Index
	SET _Priority_key = NEW._Priority_key,
    	_ConditionalMutants_key = NEW._ConditionalMutants_key 
	WHERE _Refs_key = NEW._Refs_key
	;
END IF;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.gxd_orderallelepairs(v_genotypekey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_pkey int;		-- primary key of records to update
v_oldSeq int;		-- current sequence number
v_newSeq int := 1;	-- new sequence number
BEGIN
-- NAME: GXD_orderAllelePairs
-- DESCRIPTION:
--        
-- Re-order the Allele Pairs of the given Genotype
-- alphabetically by Marker Symbol
-- if the Genotype contains an Allele whose Compound value != Not Applicable
-- then do not reorder
-- INPUT:
--      
-- v_genotypeKey : GXD_Genotype._Genotype_key
-- RETURNS:
--	VOID
--      
IF EXISTS (SELECT 1 FROM GXD_AllelePair ap, VOC_Term t
	WHERE ap._Genotype_key = v_genotypeKey
	AND ap._Compound_key = t._Term_key
	AND t.term != 'Not Applicable')
THEN
    RETURN;
END IF;
FOR v_pkey, v_oldSeq IN
SELECT _AllelePair_key, sequenceNum
FROM GXD_AllelePair_View
WHERE _Genotype_key = v_genotypeKey 
ORDER BY symbol
LOOP
  	UPDATE GXD_AllelePair 
	SET sequenceNum = v_newSeq
    	WHERE _AllelePair_key = v_pkey
	;
  	v_newSeq := v_newSeq + 1;
END LOOP;
RETURN;
END;
$$;


CREATE FUNCTION mgd.gxd_ordergenotypes(v_allelekey integer, v_userkey integer DEFAULT 1001) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_pkey int;	-- primary key of records to UPDATE 
v_oldSeq int;	-- current sequence number
v_newSeq int;	-- new sequence number
BEGIN
-- NAME: GXD_orderGenotypes
-- DESCRIPTION:
--        
-- Load the GXD_AlleleGenotype (cache) table for the given Allele.
-- Executed after any modification to GXD_AllelePair.
-- test: 263, 8734
-- INPUT:
-- v_alleleKey : ALL_Allele._Allele_key
-- v_userKey   : MGI_User._User_key = 1001 (DEFAULT)
--      
-- RETURNS:
--	VOID
--      
-- Delete any pre-existing cache results for given Allele
DELETE FROM GXD_AlleleGenotype WHERE _Allele_key = v_alleleKey;
CREATE TEMP TABLE genotypes_tmp1 ON COMMIT DROP
AS SELECT DISTINCT ap._Genotype_key, ap._Allele_key_1, s.strain
FROM GXD_AllelePair ap, GXD_Genotype g, PRB_Strain s
WHERE (ap._Allele_key_1 = v_alleleKey or ap._Allele_key_2 = v_alleleKey)
AND ap._Genotype_key = g._Genotype_key
AND g._Strain_key = s._Strain_key
ORDER BY ap._Allele_key_1, s.strain
;
CREATE TEMP TABLE genotypes_tmp ON COMMIT DROP
AS SELECT DISTINCT _Genotype_key
FROM genotypes_tmp1
;
CREATE INDEX idx_genotypes_tmp ON genotypes_tmp(_Genotype_key);
CREATE TEMP TABLE simple_tmp ON COMMIT DROP
AS SELECT DISTINCT g._Genotype_key
FROM genotypes_tmp g, GXD_AllelePair ap
WHERE g._Genotype_key = ap._Genotype_key
GROUP by g._Genotype_key having count(*) = 1
;
CREATE INDEX idx_simple_tmp ON simple_tmp(_Genotype_key);
CREATE TEMP TABLE complex_tmp ON COMMIT DROP
AS SELECT DISTINCT g._Genotype_key
FROM genotypes_tmp g, GXD_AllelePair ap
WHERE g._Genotype_key = ap._Genotype_key
GROUP by g._Genotype_key having count(*) > 1
;
CREATE INDEX idx_complex_tmp ON complex_tmp(_Genotype_key);
-- simple
--RAISE NOTICE 'insert simple';
INSERT INTO GXD_AlleleGenotype 
(_Genotype_key, _Marker_key, _Allele_key, sequenceNum, _CreatedBy_key, _ModifiedBy_key)
SELECT DISTINCT ap._Genotype_key, ap._Marker_key, ap._Allele_key_1, -1, v_userKey, v_userKey
FROM simple_tmp s, GXD_AllelePair ap
WHERE s._Genotype_key = ap._Genotype_key
AND ap._Allele_key_1 = v_alleleKey
UNION
SELECT distinct ap._Genotype_key, ap._Marker_key, ap._Allele_key_2, -1, v_userKey, v_userKey
FROM simple_tmp s, GXD_AllelePair ap
WHERE s._Genotype_key = ap._Genotype_key
AND ap._Allele_key_2 = v_alleleKey
;
-- Apply ordering rules
--AND t.term = 'Homozygous'
--RAISE NOTICE 'update 1';
UPDATE GXD_AlleleGenotype
SET sequenceNum = 1
FROM simple_tmp s, GXD_AllelePair ap
WHERE s._Genotype_key = GXD_AlleleGenotype._Genotype_key
AND GXD_AlleleGenotype._Allele_key = v_alleleKey
AND s._Genotype_key = ap._Genotype_key
AND GXD_AlleleGenotype._Allele_key = ap._Allele_key_1
AND ap._PairState_key = 847138
;
--AND t.term = 'Heterozygous'
--RAISE NOTICE 'update 2';
UPDATE GXD_AlleleGenotype
SET sequenceNum = 3
FROM simple_tmp s, GXD_AllelePair ap
WHERE s._Genotype_key = GXD_AlleleGenotype._Genotype_key
AND GXD_AlleleGenotype._Allele_key = v_alleleKey
AND s._Genotype_key = ap._Genotype_key
AND ap._PairState_key = 847137
AND exists (SELECT 1 FROM ALL_Allele a WHERE ap._Allele_key_1 = a._Allele_key and a.isWildType = 0)
;
--RAISE NOTICE 'update 3';
--AND t.term = 'Heterozygous'
UPDATE GXD_AlleleGenotype
SET sequenceNum = 3
FROM simple_tmp s, GXD_AllelePair ap
WHERE s._Genotype_key = GXD_AlleleGenotype._Genotype_key
AND GXD_AlleleGenotype._Allele_key = v_alleleKey
AND s._Genotype_key = ap._Genotype_key
AND ap._PairState_key = 847137
AND exists (SELECT 1 FROM ALL_Allele a WHERE ap._Allele_key_2 = a._Allele_key and a.isWildType = 0)
;
--RAISE NOTICE 'update 4';
--AND t.term = 'Heterozygous'
UPDATE GXD_AlleleGenotype
SET sequenceNum = 2
FROM simple_tmp s, GXD_AllelePair ap
WHERE s._Genotype_key = GXD_AlleleGenotype._Genotype_key
AND GXD_AlleleGenotype._Allele_key = v_alleleKey
AND s._Genotype_key = ap._Genotype_key
AND ap._PairState_key = 847137
AND exists (SELECT 1 FROM ALL_Allele a WHERE ap._Allele_key_1 = a._Allele_key and a.isWildType = 1)
;
--RAISE NOTICE 'update 5';
--AND t.term = 'Heterozygous'
UPDATE GXD_AlleleGenotype
SET sequenceNum = 2
FROM simple_tmp s, GXD_AllelePair ap
WHERE s._Genotype_key = GXD_AlleleGenotype._Genotype_key
AND GXD_AlleleGenotype._Allele_key = v_alleleKey
AND s._Genotype_key = ap._Genotype_key
AND ap._PairState_key = 847137
AND exists (SELECT 1 FROM ALL_Allele a WHERE ap._Allele_key_2 = a._Allele_key and a.isWildType = 1)
;
--RAISE NOTICE 'update 6';
--AND t.term = 'Hemizygous X-linked'
UPDATE GXD_AlleleGenotype
SET sequenceNum = 4
FROM simple_tmp s, GXD_AllelePair ap
WHERE s._Genotype_key = GXD_AlleleGenotype._Genotype_key
AND GXD_AlleleGenotype._Allele_key = v_alleleKey
AND s._Genotype_key = ap._Genotype_key
AND GXD_AlleleGenotype._Allele_key = ap._Allele_key_1
AND ap._PairState_key = 847133
;
--RAISE NOTICE 'update 7';
--AND t.term = 'Hemizygous Y-linked'
UPDATE GXD_AlleleGenotype
SET sequenceNum = 5
FROM simple_tmp s, GXD_AllelePair ap
WHERE s._Genotype_key = GXD_AlleleGenotype._Genotype_key
AND GXD_AlleleGenotype._Allele_key = v_alleleKey
AND s._Genotype_key = ap._Genotype_key
AND GXD_AlleleGenotype._Allele_key = ap._Allele_key_1
AND ap._PairState_key = 847134
;
--RAISE NOTICE 'update 8';
--AND t.term = 'Hemizygous Insertion'
UPDATE GXD_AlleleGenotype
SET sequenceNum = 6
FROM simple_tmp s, GXD_AllelePair ap
WHERE s._Genotype_key = GXD_AlleleGenotype._Genotype_key
AND GXD_AlleleGenotype._Allele_key = v_alleleKey
AND s._Genotype_key = ap._Genotype_key
AND GXD_AlleleGenotype._Allele_key = ap._Allele_key_1
AND ap._PairState_key = 847135
;
--RAISE NOTICE 'update 9';
--AND t.term = 'Indeterminate'
UPDATE GXD_AlleleGenotype
SET sequenceNum = 7
FROM simple_tmp s, GXD_AllelePair ap
WHERE s._Genotype_key = GXD_AlleleGenotype._Genotype_key
AND GXD_AlleleGenotype._Allele_key = v_alleleKey
AND s._Genotype_key = ap._Genotype_key
AND GXD_AlleleGenotype._Allele_key = ap._Allele_key_1
AND ap._PairState_key = 847139
;
-- complex 
--RAISE NOTICE 'insert complex';
INSERT INTO GXD_AlleleGenotype 
(_Genotype_key, _Marker_key, _Allele_key, sequenceNum, _CreatedBy_key, _ModifiedBy_key)
SELECT DISTINCT ap._Genotype_key, ap._Marker_key, ap._Allele_key_1, 8, v_userKey, v_userKey
FROM complex_tmp x, GXD_AllelePair ap
WHERE x._Genotype_key = ap._Genotype_key
AND ap._Allele_key_1 = v_alleleKey
UNION
SELECT DISTINCT ap._Genotype_key, ap._Marker_key, ap._Allele_key_2, 8, v_userKey, v_userKey
FROM complex_tmp x, GXD_AllelePair ap
WHERE x._Genotype_key = ap._Genotype_key
AND ap._Allele_key_2 = v_alleleKey
;
-- re-sequence to get rid of duplicate sequence numbers/gaps
v_newSeq := 1;
FOR v_pkey, v_oldSeq IN
SELECT ap._Genotype_key, ap.sequenceNum, s.strain
FROM GXD_AlleleGenotype ap, GXD_Genotype g, PRB_Strain s
WHERE ap._Allele_key = v_alleleKey
AND ap._Genotype_key = g._Genotype_key
AND g._Strain_key = s._Strain_key
ORDER BY ap.sequenceNum, s.strain
LOOP
	--RAISE NOTICE 're-sequence %:%:%', v_alleleKey, v_pkey, v_newSeq;
	UPDATE GXD_AlleleGenotype 
	SET sequenceNum = v_newSeq 
	WHERE _Genotype_key = v_pkey 
	AND _Allele_key = v_alleleKey
	;
	v_newSeq := v_newSeq + 1;
END LOOP;
 
-- Drop temp tables
DROP TABLE IF EXISTS genotypes_tmp1;
DROP TABLE IF EXISTS genotypes_tmp;
DROP TABLE IF EXISTS simple_tmp;
DROP TABLE IF EXISTS complex_tmp;
RETURN;
END;
$$;


CREATE FUNCTION mgd.gxd_ordergenotypesall(v_genotypekey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_alleleKey int;
BEGIN
-- NAME: GXD_orderGenotypesAll
-- DESCRIPTION:
--        
-- Refrest the GXD_AlleleGenotype (cache) table for all Genotypes
-- INPUT:
--      
-- v_genotypeKey : GXD_Genotype._Genotype_key
-- RETURNS:
--	VOID
--      
DELETE FROM GXD_AlleleGenotype WHERE _Genotype_key = v_genotypeKey;
FOR v_alleleKey IN
SELECT DISTINCT _Allele_key_1 
FROM GXD_AllelePair
WHERE _Genotype_key = v_genotypeKey
UNION
SELECT DISTINCT _Allele_key_2 
FROM GXD_AllelePair
WHERE _Genotype_key = v_genotypeKey
AND _Allele_key_2 IS NOT NULL
LOOP
  	PERFORM GXD_orderGenotypes (v_alleleKey, 1001);
END LOOP;
 
RETURN;
END;
$$;


CREATE FUNCTION mgd.gxd_ordergenotypesmissing() RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_alleleKey int;
BEGIN
-- NAME: GXD_orderGenotypesMissing
-- DESCRIPTION:
--        
-- Refresh the GXD_AlleleGenotype (cache) table for all Genotypes
-- which are missing
-- INPUT:
--	None
-- RETURNS:
--	VOID
--      
FOR v_alleleKey IN
SELECT DISTINCT _Allele_key_1 
FROM GXD_AllelePair a
WHERE not exists 
(SELECT 1 FROM GXD_AlleleGenotype g 
WHERE a._Genotype_key = g._Genotype_key
AND a._Allele_key_1 = g._Allele_key)
UNION
SELECT DISTINCT _Allele_key_2 
FROM GXD_AllelePair a
WHERE a._Allele_key_2 IS NOT NULL
AND NOT EXISTS 
(SELECT 1 FROM GXD_AlleleGenotype g 
WHERE a._Genotype_key = g._Genotype_key
AND a._Allele_key_2 = g._Allele_key)
LOOP
 	PERFORM GXD_orderGenotypes (v_alleleKey);
END LOOP;
END;
$$;


CREATE FUNCTION mgd.gxd_removebadgelband() RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: GXD_removeBadGelBand
-- DESCRIPTION:
--        
-- To delete Gel Bands that are not associated with any Gel Rows
-- INPUT:
--	None
--      
-- RETURNS:
--	VOID
--      
CREATE TEMP TABLE tmp_band ON COMMIT DROP
AS (select distinct _Assay_key from GXD_GelRow);
CREATE TEMP TABLE tmp_delete ON COMMIT DROP
AS (select tmp_band.*, b.*
from tmp_band, GXD_GelBand b, GXD_GelRow r, GXD_GelLane l
WHERE b._GelLane_key = l._GelLane_key
AND r._GelRow_key = b._GelRow_key
AND r._Assay_key != l._Assay_key
AND r._Assay_key = tmp_band._Assay_key
)
;
DELETE FROM GXD_GelBand
USING tmp_delete
WHERE tmp_delete._GelBand_key = GXD_GelBand._GelBand_key
;
END;
$$;


CREATE FUNCTION mgd.gxd_replacegenotype(v_createdby text, v_refskey integer, v_oldgenotypekey integer, v_newgenotypekey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: GXD_replaceGenotype
-- DESCRIPTION:
--        
-- To replace old Genotype with new Genotype
-- in GXD_Specimen, GXD_GelLane, GXD_Expression, GXD_Assay
-- INPUT:
--      
-- v_createdBy : MGI_User.name
-- v_refsKey : BIB_Refs._Refs_key
-- v_oldGenotypeKey : GXD_Genotype._Genotype_key
-- v_newGenotypeKey : GXD_Genotype._Genotype_key
-- RETURNS:
--	VOID
--      
UPDATE GXD_Specimen s
SET _Genotype_key = v_newGenotypeKey
FROM GXD_Assay a
WHERE a._Refs_key = v_refsKey
and a._Assay_key = s._Assay_key
and s._Genotype_key = v_oldGenotypeKey
;
UPDATE GXD_GelLane s
SET _Genotype_key = v_newGenotypeKey
FROM GXD_Assay a
WHERE a._Refs_key = v_refsKey
and a._Assay_key = s._Assay_key
and s._Genotype_key = v_oldGenotypeKey
;
UPDATE GXD_Expression s
SET _Genotype_key = v_newGenotypeKey
WHERE s._Refs_key = v_refsKey
and s._Assay_key = s._Assay_key
and s._Genotype_key = v_oldGenotypeKey
;
UPDATE GXD_Assay a
SET _ModifiedBy_key = u._user_key, modification_date = now()
FROM MGI_User u
WHERE a._Refs_key = v_refsKey
and v_createdBy = u.login
;
END;
$$;


CREATE FUNCTION mgd.img_image_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: IMG_Image_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INCLUDED RULES:
--	Deletes Thumbnail Images, as needed
-- INPUT:
--	none
-- RETURNS:
--	NEW
DELETE FROM IMG_ImagePane_Assoc a
WHERE a._Object_key = OLD._Image_key
AND a._MGIType_key = 9
;
DELETE FROM MAP_Coord_Feature a
WHERE a._Object_key = OLD._Image_key
AND a._MGIType_key = 9
;
DELETE FROM MAP_Coordinate a
WHERE a._Object_key = OLD._Image_key
AND a._MGIType_key = 9
;
DELETE FROM MGI_Note a
WHERE a._Object_key = OLD._Image_key
AND a._MGIType_key = 9
;
DELETE FROM MGI_Property a
WHERE a._Object_key = OLD._Image_key
AND a._MGIType_key = 9
;
DELETE FROM MGI_Reference_Assoc a
WHERE a._Object_key = OLD._Image_key
AND a._MGIType_key = 9
;
DELETE FROM MGI_Synonym a
WHERE a._Object_key = OLD._Image_key
AND a._MGIType_key = 9
;
DELETE FROM ACC_Accession a
WHERE a._Object_key = OLD._Image_key
AND a._MGIType_key = 9
;
-- EMAGE ids
DELETE FROM ACC_Accession a
WHERE a._Object_key = OLD._ThumbnailImage_key
AND a._MGIType_key = 35
;
-- if this is a Full Size Image, then delete its Thumbnail as well
IF (OLD._ImageType_key = 1072158)
THEN
    DELETE FROM IMG_Image a
    where OLD._ThumbnailImage_key = a._Image_key
    ;
    DELETE FROM MGI_Note a
    WHERE a._MGIType_key = 9
    AND a._Object_key = OLD._ThumbnailImage_key
    ;
    DELETE FROM IMG_ImagePane a
    WHERE a._Image_key = OLD._ThumbnailImage_key
    ;
 
    DELETE FROM ACC_Accession a
    WHERE a._Object_key = OLD._ThumbnailImage_key
    AND a._MGIType_key = 9
    ;
    -- EMAGE ids
    DELETE FROM ACC_Accession a
    WHERE a._Object_key = OLD._ThumbnailImage_key
    AND a._MGIType_key = 35
    ;
END IF;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.img_image_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: IMG_Image_insert()
-- DESCRIPTOIN:
--	this insert trigger will call ACC_assignMGI
--	in order to add a distinct MGI accession id
--	to the NEW object
-- INPUT:
--	none
-- RETURNS:
--	NEW
PERFORM ACC_assignMGI(1001, NEW._Image_key, 'Image');
RETURN NEW;
END;
$$;


-- CREATE FUNCTION mgd.img_setpdo(v_pixid integer, v_xdim integer, v_ydim integer, v_image_key integer) RETURNS void
--     LANGUAGE plpgsql
--     AS $$
-- DECLARE
-- v_accID acc_accession.accID%TYPE;
-- v_prefix varchar(4);
-- v_imageLDB int;
-- v_imageType int;
-- BEGIN
-- -- NAME: IMG_setPDO
-- -- DESCRIPTION:
-- --        
-- -- adds the PIX id to the MGI Image (ACC_Accession, IMG_Image)
-- -- set the IMG_Image.xDim, yDim values
-- -- If image_key is valid and a PIX foreign accession number
-- -- does not already exist for the _Image_key and the PIX: accession
-- -- ID does not already exist, the new ID is added to ACC_Accession
-- -- and the x,y dim update the image record.
-- -- ASSUMES:
-- -- - _LogicalDB_key for "MGI Image Archive" is 19,
-- -- - _MGIType_key for mgi Image objects is 9
-- -- REQUIRES:
-- -- - four integer inputs
-- -- - _Image_key exists
-- -- - _Image_key is not referenced by an existing PIX:#
-- -- - PIX:# does not exist (referencing another _Image_key)
-- -- INPUT:
-- -- v_pixID     : ACC_Accession.accID
-- -- v_xDim      : IMG_Image.xDmin
-- -- v_yDim      : IMG_Image.yDmin
-- -- v_image_key : IMG_Image._Image_key
-- -- RETURNS:
-- --	VOID
-- v_prefix := 'PIX:'; 
-- v_imageLDB := 19;
-- v_imageType := 9;
-- IF v_pixID IS NULL OR v_image_key IS NULL OR v_xDim IS NULL OR v_yDim IS NULL
-- THEN
-- 	RAISE EXCEPTION E'IMG_setPDO: All four arguments must be non-NULL.';
-- 	RETURN;
-- ELSE
-- 	v_accID := v_prefix || v_pixID;
-- END IF;
-- -- ck for missing image rec
-- IF NOT EXISTS (SELECT 1 FROM IMG_Image WHERE _Image_key = v_image_key)
-- THEN
-- 	RAISE EXCEPTION E'IMG_setPDO: % _Image_key does not exist.', v_image_key;
-- 	RETURN;
-- END IF;
-- -- check that this PIX:# does not already exist
-- IF EXISTS (SELECT 1 FROM ACC_Accession 
--    WHERE accID = v_accID AND _MGIType_key = v_imageType
--    AND _LogicalDB_key = v_imageLDB 
--    )
-- THEN
-- 	RAISE EXCEPTION E'IMG_setPDO: % accession already exists.', v_accID;
-- 	RETURN;
-- END IF;
-- -- check that image record is not referenced by another PIX:#
-- IF EXISTS (SELECT 1 FROM ACC_Accession
--    WHERE _Object_key = v_image_key AND prefixPart = v_prefix
--    AND _LogicalDB_key = v_imageLDB AND _MGIType_key = v_imageType
--    )
-- THEN
-- 	RAISE EXCEPTION E'IMG_setPDO: A PIX: accession already exists for _Image_key %.', v_image_key;
-- 	RETURN;
-- END IF;
-- -- insert the new PIX: accession record 
-- PERFORM ACC_insert (1001, v_image_key, v_accID, v_imageLDB, 'Image', -1, 1, 1);
-- IF NOT FOUND
-- THEN
-- 	RAISE EXCEPTION E'IMG_setPDO: ACC_insert failed.';
-- 	RETURN;
-- END IF;
-- -- set the image dimensions 
-- UPDATE IMG_Image 
-- SET xDim = v_xDim, yDim = v_yDim
-- WHERE _Image_key = v_image_key
-- ;
-- IF NOT FOUND
-- THEN
--         RAISE EXCEPTION E'IMG_setPDO: Update x,y Dimensions failed.';
--         RETURN;
-- END IF;
-- END;
-- $$;


CREATE FUNCTION mgd.mgi_addsetmember(v_setkey integer, v_objectkey integer, v_userkey integer, v_label text, v_stagekey integer DEFAULT 0) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_newSetMemberKey int;
v_newSeqKey int;
v_newSetMemberEmapaKey int;
BEGIN
-- NAME: MGI_addSetMember
-- DESCRIPTION:
-- To add set member to set
-- (MGI_SetMember, MGI_SetMember_Genotype)
--        
-- INPUT:
--      
-- v_setKey      : MGI_Set._Set_key
-- v_objectKey   : should match v_setKey
-- v_userKey     : MGI_User._User_key
-- v_label       : MGI_SetMember.label
-- v_stageKey    : GXD_TheilerStage._Stage_key
-- RETURNS:
--	VOID
v_newSetMemberKey := max(_SetMember_key) + 1 from MGI_SetMember;
v_newSeqKey := max(sequenceNum) + 1 from MGI_SetMember where _Set_key = v_setKey and _CreatedBy_key = v_userKey;
v_newSetMemberEmapaKey := max(_SetMember_Emapa_key) + 1 from MGI_SetMember_EMAPA;
IF v_newSeqKey is null THEN
	v_newSeqKey := 1;
END IF;
-- if _set_key, _object_key, _createdby_key does not exists, then add it
IF v_setKey != 1046 AND 
        NOT EXISTS (select * from MGI_SetMember where _set_key = v_setKey and _object_key = v_objectKey and _createdby_key = v_userKey) THEN
        INSERT INTO MGI_SetMember
        SELECT v_newSetMemberKey, v_setKey, v_objectKey, v_label, v_newSeqKey, v_userKey, v_userKey, now(), now()
        ;
        RETURN;
END IF;
-- if _set_key, _object_key, _createdby_key, _stage_key does not exists, then add it
-- for sets that require MGI_SetMember_EMAPA
IF v_setKey = 1046 AND
        NOT EXISTS (select * from MGI_SetMember s, MGI_SetMember_EMAPA e
                where s._set_key = v_setKey 
                and s._object_key = v_objectKey 
                and s._createdby_key = v_userKey
                and s._setmember_key = e._setmember_key
                and e._stage_key = v_stageKey) THEN
	        INSERT INTO MGI_SetMember
	        SELECT v_newSetMemberKey, v_setKey, v_objectKey, v_label, v_newSeqKey, v_userKey, v_userKey, now(), now()
	        ;
                INSERT INTO MGI_SetMember_EMAPA
                SELECT v_newSetMemberEmapaKey, v_newSetMemberKey, v_stageKey, v_userKey, v_userKey, now(), now()
                ;
        RETURN;
END IF;
END;
$$;


CREATE FUNCTION mgd.mgi_checkemapaclipboard(v_setmember_key integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_stage_key int;		/* clipboard stage key */
v_emapa_term_key int;	/* clipboard EMAPA _term_key */
v_startstage int;		/* clipboard EMAPA startstage for term */
v_endstage int;		/* clipboard EMAPA endstage for term */
BEGIN
/* Check if this is EMAPA/stage set */
IF 1046 != (select _set_key from MGI_SetMember where _setmember_key = v_setmember_key)
THEN
	return;
END IF;
/* Check for matching stage record */
v_stage_key = (select _stage_key
	from MGI_SetMember_EMAPA
	where _setmember_key = v_setmember_key);
IF v_stage_key is NULL THEN
	return;	
END IF;
/* Check for matching EMAPA record */
/* gather stage range variables */
select _term_key, startstage, endstage
    into v_emapa_term_key, v_startstage, v_endstage
    from VOC_Term_EMAPA vte
        join MGI_SetMember msm on
            msm._object_key = vte._term_key
    where msm._setmember_key = v_setmember_key;
IF v_emapa_term_key is NULL THEN
	return;	
END IF;
/* Check for valid stage range */
IF v_stage_key < v_startstage OR v_stage_key > v_endstage THEN
    RAISE EXCEPTION E'Invalid clipboard stage %, outside range of %-% for _emapa_term_key=: %',
	v_stage_key, v_startstage, v_endstage, v_emapa_term_key;
END IF;
END;
$$;


CREATE FUNCTION mgd.mgi_cleannote(v_notekey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
update MGI_Note 
set note = replace(note,E'\r','') 
where _note_key = v_noteKey
and note like E'%\r%'
;
END;
$$;


CREATE FUNCTION mgd.mgi_deleteemapaclipboarddups(v_setmember_key integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_stage_key int;	/* clipboard stage key */
v_emapa_term_key int;	/* clipboard EMAPA _term_key */
v_user_key int;		/* clipboard _createdby_key */
BEGIN
/* Check if this is EMAPA/stage set */
IF 1046 != (select _set_key from MGI_SetMember where _setmember_key = v_setmember_key)
THEN
	return;
END IF;
/* Check for matching stage record */
v_stage_key = (select _stage_key
	from MGI_SetMember_EMAPA
	where _setmember_key = v_setmember_key);
IF v_stage_key is NULL THEN
	return;	
END IF;
/* gather the _emapa_term_key and _user_key for this set member */
select _object_key, _createdby_key
into v_emapa_term_key, v_user_key
from MGI_SetMember
where _setmember_key = v_setmember_key
;
/* Delete all old occurrences of the new record's term/stage combo */
delete from MGI_SetMember old
using MGI_SetMember_EMAPA old_stage
where old_stage._setmember_key = old._setmember_key
  -- same _createdby_key
  and old._createdby_key = v_user_key
  -- same stage
  and old_stage._stage_key = v_stage_key
  -- same term
  and old._object_key = v_emapa_term_key
  -- but not the new record
  and old._setmember_key != v_setmember_key
;
END;
$$;


CREATE FUNCTION mgd.mgi_insertreferenceassoc(v_userkey integer, v_mgitypekey integer, v_objectkey integer, v_refskey integer, v_refstypekey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_assocKey int;
BEGIN
-- Insert record into MGI_Reference_Assoc 
-- if the reference does not already exist
-- antibody
IF v_mgiTypeKey = 6 
THEN
	-- for antibody, do not include the reference type 
	-- as part of the check for duplicate references
	IF exists (SELECT 1 FROM MGI_Reference_Assoc
		WHERE _Refs_key = v_refsKey
		AND _Object_key = v_objectKey
		AND _MGIType_key = v_mgiTypeKey)
	THEN
		RETURN;
	END IF;
ELSE
	-- for all other mgi types, include the reference type
	-- as part of the check for duplicate references
	IF EXISTS (SELECT 1 FROM MGI_Reference_Assoc
		WHERE _Refs_key = v_refsKey
		AND _Object_key = v_objectKey
		AND _MGIType_key = v_mgiTypeKey
		AND _RefAssocType_key = v_refsTypeKey)
	THEN
		RETURN;
	END IF;
END IF;
v_assocKey := nextval('mgi_reference_assoc_seq');
INSERT INTO MGI_Reference_Assoc
VALUES (v_assocKey, v_refsKey, v_objectKey, v_mgiTypeKey, v_refsTypeKey,
	v_userKey, v_userKey, now(), now())
;
-- if the v_refsKey does not have a J:, then add it
IF NOT EXISTS (SELECT 1 FROM ACC_Accession 
        WHERE _Object_key = v_refsKey
        AND _LogicalDB_key = 1
        AND _MGIType_key = 1
        AND prefixPart = 'J:')
THEN
        PERFORM ACC_assignJ(1001, v_refsKey, -1);
END IF;
END;
$$;


CREATE FUNCTION mgd.mgi_insertsynonym(v_userkey integer, v_objectkey integer, v_mgitypekey integer, v_syntypekey integer, v_synonym text, v_refskey integer DEFAULT NULL::integer, v_ignoreduplicate integer DEFAULT 1) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_synKey int;
BEGIN
v_synKey := nextval('mgi_synonym_seq');
IF NOT EXISTS (SELECT 1 FROM MGI_Synonym
	WHERE _Object_key = v_objectKey
	AND _MGIType_key = v_mgiTypeKey
	AND _SynonymType_key = v_synTypeKey
	AND synonym = v_synonym)
THEN
	INSERT INTO MGI_Synonym
	(_Synonym_key, _Object_key, _MGIType_key, _SynonymType_key, _Refs_key, synonym, 
	 _CreatedBy_key, _ModifiedBy_key, creation_date, modification_date)
	VALUES (v_synKey, v_objectKey, v_mgiTypeKey, v_synTypeKey, v_refsKey, v_synonym, 
		v_userKey, v_userKey, now(), now());
ELSE
	IF v_ignoreDuplicate = 0
	THEN
		RAISE EXCEPTION E'\nMGI_insertSynonym: Duplicate Synonym Symbol (%)', v_synonym;
		RETURN;
	END IF;
END IF;
END;
$$;


CREATE FUNCTION mgd.mgi_mergerelationship(v_oldkey integer, v_newkey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
-- Participant relationship (any category)
-- _Objec_key_1 anything else
-- _Objec_key_2 is Marker (_MGIType_key = 2)
-- if old marker *does* contain same relationship as new marker, then delete old marker relationship
-- that is, we do not want a duplicate marker relationship after the merge
delete 
from MGI_Relationship
using MGI_Relationship r_new
where MGI_Relationship._Object_key_2 = v_oldKey
and r_new._Object_key_2 = v_newKey
and MGI_Relationship._Category_key = r_new._Category_key
and MGI_Relationship._Object_key_1 = r_new._Object_key_1
and MGI_Relationship._RelationshipTerm_key = r_new._RelationshipTerm_key
and MGI_Relationship._Qualifier_key = r_new._Qualifier_key
and MGI_Relationship._Evidence_key = r_new._Evidence_key
and MGI_Relationship._Refs_key = r_new._Refs_key
;
-- if old marker does *not* contain same relationship with new marker, then move old marker relationship to new marker
update MGI_Relationship
set _Object_key_2 = v_newKey
where _Object_key_2 = v_oldKey
and not exists (select 1 from MGI_Relationship r_new
	where r_new._Object_key_2 = v_newKey
	and MGI_Relationship._Category_key = r_new._Category_key
	and MGI_Relationship._Object_key_1 = r_new._Object_key_1
	and MGI_Relationship._RelationshipTerm_key = r_new._RelationshipTerm_key
	and MGI_Relationship._Qualifier_key = r_new._Qualifier_key
	and MGI_Relationship._Evidence_key = r_new._Evidence_key
	and MGI_Relationship._Refs_key = r_new._Refs_key
	)
;
-- Organizer relationship only (1001:'interacts_with', 1002:'clsuter_has_member')
-- checking where:
-- _Objec_key_1 is Marker (_MGIType_key = 2)
-- _Objec_key_2 is Marker (_MGIType_key = 2)
-- if old marker *does* contain same relationship as new marker, then delete old marker relationship
-- that is, we do not want a duplicate marker relationship after the merge
delete 
from MGI_Relationship
using MGI_Relationship r_new
where MGI_Relationship._Object_key_1 = v_oldKey
and MGI_Relationship._Category_key in (1001, 1002)
and r_new._Object_key_1 = v_newKey
and MGI_Relationship._Category_key = r_new._Category_key
and MGI_Relationship._Object_key_2 = r_new._Object_key_2
and MGI_Relationship._RelationshipTerm_key = r_new._RelationshipTerm_key
and MGI_Relationship._Qualifier_key = r_new._Qualifier_key
and MGI_Relationship._Evidence_key = r_new._Evidence_key
and MGI_Relationship._Refs_key = r_new._Refs_key
;
-- if old marker does *not* contain same relationship with new marker, then move old marker relationship to new marker
update MGI_Relationship
set _Object_key_1 = v_newKey
where _Object_key_1 = v_oldKey
and _Category_key in (1001, 1002)
and not exists (select 1 from MGI_Relationship r_new
	where r_new._Object_key_1 = v_newKey
        and r_new._Category_key in (1001, 1002)
	and MGI_Relationship._Category_key = r_new._Category_key
	and MGI_Relationship._Object_key_2 = r_new._Object_key_2
	and MGI_Relationship._RelationshipTerm_key = r_new._RelationshipTerm_key
	and MGI_Relationship._Qualifier_key = r_new._Qualifier_key
	and MGI_Relationship._Evidence_key = r_new._Evidence_key
	and MGI_Relationship._Refs_key = r_new._Refs_key
	)
;
END;
$$;


CREATE FUNCTION mgd.mgi_organism_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: MGI_Organism_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INPUT:
--	none
-- RETURNS:
--	NEW
DELETE FROM IMG_ImagePane_Assoc a
WHERE a._Object_key = OLD._Organism_key
AND a._MGIType_key = 20
;
DELETE FROM MAP_Coord_Feature a
WHERE a._Object_key = OLD._Organism_key
AND a._MGIType_key = 20
;
DELETE FROM MAP_Coordinate a
WHERE a._Object_key = OLD._Organism_key
AND a._MGIType_key = 20
;
DELETE FROM MGI_Note a
WHERE a._Object_key = OLD._Organism_key
AND a._MGIType_key = 20
;
DELETE FROM MGI_Property a
WHERE a._Object_key = OLD._Organism_key
AND a._MGIType_key = 20
;
DELETE FROM MGI_Reference_Assoc a
WHERE a._Object_key = OLD._Organism_key
AND a._MGIType_key = 20
;
DELETE FROM MGI_Synonym a
WHERE a._Object_key = OLD._Organism_key
AND a._MGIType_key = 20
;
DELETE FROM ACC_Accession a
WHERE a._Object_key = OLD._Organism_key
AND a._MGIType_key = 20
;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.mgi_organism_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: MGI_Organism_insert()
-- DESCRIPTOIN:
--	this insert trigger will call ACC_assignMGI
--	in order to add a distinct MGI accession id
--	to the NEW object
-- INPUT:
--	none
-- RETURNS:
--	NEW
PERFORM ACC_assignMGI(1001, NEW._Organism_key, 'Organism');
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.mgi_processnote(v_userkey integer, v_note_key integer, v_objectkey integer, v_mgitypekey integer, v_notetypekey integer, v_note text) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_noteKey int;
v_existingnoteTypeKey int;
BEGIN
v_existingnoteTypeKey :=  _notetype_key from MGI_Note where _note_key = v_note_key;
--raise exception 'v_existingnoteTypeKey: %', v_existingnoteTypeKey;
--raise exception 'v_noteTypeKey: %', v_noteTypeKey;
IF v_note_key IS NULL
THEN
        v_noteKey := nextval('mgi_note_seq');
	INSERT INTO MGI_Note
	(_Note_key, _Object_key, _MGIType_key, _NoteType_key, note,
	 _CreatedBy_key, _ModifiedBy_key, creation_date, modification_date)
	VALUES (v_noteKey, v_objectKey, v_mgiTypeKey, v_noteTypeKey, v_note,
		v_userKey, v_userKey, now(), now());
ELSEIF v_note IS NULL or v_note = 'null'
THEN
	DELETE FROM MGI_Note where _Note_key = v_note_key;
ELSE
	UPDATE MGI_Note
	SET note = v_note,
        _NoteType_key = v_noteTypeKey,
	_ModifiedBy_key = v_userKey, 
	modification_date = now()
	WHERE _Note_key = v_note_key;
END IF;
END;
$$;


CREATE FUNCTION mgd.mgi_reference_assoc_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: MGI_Reference_Assoc_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INPUT:
--	none
-- RETURNS:
--	NEW
-- Antibody objects ....
IF EXISTS (SELECT 1 FROM GXD_AntibodyPrep p, GXD_Assay a
           WHERE OLD._MGIType_key = 6 
           AND OLD._Object_key = p._Antibody_key
           AND p._AntibodyPrep_key = a._AntibodyPrep_key
           AND OLD._Refs_key = a._Refs_key)
THEN
        RAISE EXCEPTION E'You cannot delete an Antibody Reference that is cross-referenced to an Assay.';
END IF;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.mgi_reference_assoc_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: MGI_Reference_Assoc_insert()
-- DESCRIPTION:
--      1) if the Reference is for an Allele and exists as group = 'AP'
--              in BIB_Workflow_Status, and the status is not 'Full-coded'
--         then
--              set existing BIB_Workflow_Status.isCurrent = 0
--              add new BIB_Workflow_Status._Status_key = 'Indexed'
-- changes to this trigger require changes to procedure/BIB_updateWFStatusAP_create.object
-- INPUT:
--      none
-- RETURNS:
--      NEW
-- Allele objects ...
IF NEW._MGIType_key = 11
AND EXISTS (SELECT 1 FROM BIB_Workflow_Status b
	WHERE NEW._Refs_key = b._Refs_key
	AND b._Group_key = 31576664
	AND b._Status_key != 31576674
	AND b.isCurrent = 1)
THEN
    UPDATE BIB_Workflow_Status w set isCurrent = 0
    WHERE NEW._Refs_key = w._Refs_key
	AND w._Group_key = 31576664
    ;
    INSERT INTO BIB_Workflow_Status
    VALUES((select nextval('bib_workflow_status_seq')), NEW._Refs_key, 31576664, 31576673, 1,
	NEW._CreatedBy_key, NEW._ModifiedBy_key, now(), now())
    ;
    PERFORM BIB_keepWFRelevance (NEW._Refs_key, NEW._CreatedBy_key);
END IF;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.mgi_relationship_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: MGI_Relationship_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INPUT:
--	none
-- RETURNS:
--	NEW
DELETE FROM MGI_Note m
WHERE m._Object_key = OLD._Relationship_key
AND m._MGIType_key = 40
;
RETURN NEW;
END;
$$;


-- CREATE FUNCTION mgd.mgi_resetageminmax(v_table text, v_key integer) RETURNS void
--     LANGUAGE plpgsql
--     AS $$
-- DECLARE
-- v_pkey int; 	/* primary key of records to UPDATE */
-- v_age prb_source.age%TYPE;
-- v_ageMin numeric;
-- v_ageMax numeric;
-- age_cursor refcursor;
-- BEGIN
-- -- Update the Age Min, Age Max values
-- IF (v_table = 'GXD_Expression')
-- THEN
-- 	OPEN age_cursor FOR
-- 	SELECT _Expression_key, age
-- 	FROM GXD_Expression
-- 	WHERE _Expression_key = v_key;
-- ELSIF (v_table = 'GXD_GelLane')
-- THEN
-- 	OPEN age_cursor FOR
-- 	SELECT _GelLane_key, age
-- 	FROM GXD_GelLane
-- 	WHERE _GelLane_key = v_key;
-- ELSIF (v_table = 'GXD_Specimen')
-- THEN
-- 	OPEN age_cursor FOR
-- 	SELECT _Specimen_key, age
-- 	FROM GXD_Specimen
-- 	WHERE _Specimen_key = v_key;
-- ELSIF (v_table = 'PRB_Source')
-- THEN
-- 	OPEN age_cursor FOR
-- 	SELECT _Source_key, age
-- 	FROM PRB_Source
-- 	WHERE _Source_key = v_key;
-- ELSIF (v_table = 'GXD_HTSample')
-- THEN
-- 	OPEN age_cursor FOR
-- 	SELECT _Sample_key, age
-- 	FROM GXD_HTSample
-- 	WHERE _Sample_key = v_key;
-- ELSE
-- 	RETURN;
-- END IF;
-- LOOP
-- 	FETCH age_cursor INTO v_pkey, v_age;
-- 	EXIT WHEN NOT FOUND;
-- 	-- see PRB_ageMinMex for exceptions
-- 	SELECT * FROM PRB_ageMinMax(v_age) into v_ageMin, v_ageMax;
--         -- no ageMin/ageMax null values
--         -- commented out; there are dependcies upstream that are expecting null
--         IF (v_ageMin is null)
--         THEN
--                 v_ageMin := -1;
--                 v_ageMax := -1;
--         END IF;
-- 	IF (v_table = 'GXD_Expression')
-- 	THEN
-- 		UPDATE GXD_Expression 
-- 		SET ageMin = v_ageMin, ageMax = v_ageMax WHERE _Expression_key = v_pkey;
-- 	ELSIF (v_table = 'GXD_GelLane')
-- 	THEN
-- 		UPDATE GXD_GelLane 
-- 		SET ageMin = v_ageMin, ageMax = v_ageMax WHERE _GelLane_key = v_pkey;
-- 	ELSIF (v_table = 'GXD_Specimen')
-- 	THEN
-- 		UPDATE GXD_Specimen 
-- 		SET ageMin = v_ageMin, ageMax = v_ageMax WHERE _Specimen_key = v_pkey;
-- 	ELSIF (v_table = 'PRB_Source')
-- 	THEN
-- 		UPDATE PRB_Source 
-- 		SET ageMin = v_ageMin, ageMax = v_ageMax WHERE _Source_key = v_pkey;
-- 	ELSIF (v_table = 'GXD_HTSample')
-- 	THEN
-- 		UPDATE GXD_HTSample
-- 		SET ageMin = v_ageMin, ageMax = v_ageMax WHERE _Sample_key = v_pkey;
-- 	END IF;
-- END LOOP;
-- CLOSE age_cursor;
-- RETURN;
-- END;
-- $$;


CREATE FUNCTION mgd.mgi_resetsequencenum(v_table text, v_key integer, v_userkey integer DEFAULT 0) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
pkey int;	/* primary key of records to UPDATE */
oldSeq int;	/* current sequence number */
newSeq int;	/* new sequence number */
seq_cursor refcursor;
BEGIN
/* Re-order the sequenceNum field so that they are
continuous and there are no gaps.
ex. 1,2,5,6,7 would be reordered to 1,2,3,4,5
*/
newSeq := 1;
IF (v_table = 'GXD_AllelePair')
THEN
	OPEN seq_cursor FOR
	SELECT _AllelePair_key, sequenceNum
	FROM GXD_AllelePair
	WHERE _Genotype_key = v_key
	ORDER BY sequenceNum
	;
ELSIF (v_table = 'GXD_AlleleGenotype')
THEN
	OPEN seq_cursor FOR
	SELECT _Allele_key, sequenceNum
	FROM GXD_AlleleGenotype
	WHERE _Allele_key = v_key
	ORDER BY sequenceNum
	;
ELSIF (v_table = 'GXD_GelLane')
THEN
	OPEN seq_cursor FOR
	SELECT _GelLane_key, sequenceNum
	FROM GXD_GelLane
	WHERE _Assay_key = v_key
	ORDER BY sequenceNum
	;
ELSIF (v_table = 'GXD_GelRow')
THEN
	OPEN seq_cursor FOR
	SELECT _GelRow_key, sequenceNum
	FROM GXD_GelRow
	WHERE _Assay_key = v_key
	ORDER BY size desc
	;
ELSIF (v_table = 'GXD_Specimen')
THEN
	OPEN seq_cursor FOR
	SELECT _Specimen_key, sequenceNum
	FROM GXD_Specimen
	WHERE _Assay_key = v_key
	ORDER BY sequenceNum
	;
ELSIF (v_table = 'GXD_InSituResult')
THEN
	OPEN seq_cursor FOR
	SELECT _Result_key, sequenceNum
	FROM GXD_InSituResult
	WHERE _Specimen_key = v_key
	ORDER BY sequenceNum
	;
ELSIF (v_table = 'MRK_History')
THEN
	OPEN seq_cursor FOR
	SELECT _Marker_key, sequenceNum
	FROM MRK_History
	WHERE _Marker_key = v_key
	ORDER BY sequenceNum
	;
ELSIF (v_table = 'MLD_Expt_Marker')
THEN
	OPEN seq_cursor FOR
	SELECT _Expt_key, sequenceNum
	FROM MLD_Expt_Marker
	WHERE _Expt_key = v_key
	ORDER BY sequenceNum
	;
ELSIF (v_table = 'MLD_MCDataList')
THEN
	OPEN seq_cursor FOR
	SELECT _Expt_key, sequenceNum
	FROM MLD_MCDataList
	WHERE _Expt_key = v_key
	ORDER BY sequenceNum
	;
ELSIF (v_table = 'MLD_MC2point')
THEN
	OPEN seq_cursor FOR
	SELECT _Expt_key, sequenceNum
	FROM MLD_MC2point
	WHERE _Expt_key = v_key
	ORDER BY sequenceNum
	;
ELSIF (v_table = 'MLD_RIData')
THEN
	OPEN seq_cursor FOR
	SELECT _Expt_key, sequenceNum
	FROM MLD_RIData
	WHERE _Expt_key = v_key
	ORDER BY sequenceNum
	;
ELSIF (v_table = 'MLD_RI2Point')
THEN
	OPEN seq_cursor FOR
	SELECT _Expt_key, sequenceNum
	FROM MLD_RI2Point
	WHERE _Expt_key = v_key
	ORDER BY sequenceNum
	;
ELSIF (v_table = 'MLD_FISH_Region')
THEN
	OPEN seq_cursor FOR
	SELECT _Expt_key, sequenceNum
	FROM MLD_FISH_Region
	WHERE _Expt_key = v_key
	ORDER BY sequenceNum
	;
ELSIF (v_table = 'MRK_Chromosome')
THEN
	OPEN seq_cursor FOR
	SELECT _Chromosome_key, sequenceNum
	FROM MRK_Chromosome
	WHERE _Organism_key = v_key
	ORDER BY sequenceNum
	;
ELSIF (v_table = 'MLD_Statistics')
THEN
	OPEN seq_cursor FOR
	SELECT _Expt_key, sequenceNum
	FROM MLD_Statistics
	WHERE _Expt_key = v_key
	ORDER BY sequenceNum
	;
ELSIF (v_table = 'MGI_Translation')
THEN
	OPEN seq_cursor FOR
	SELECT _TranslationType_key, sequenceNum
	FROM MGI_Translation
	WHERE _TranslationType_key = v_key
	ORDER BY sequenceNum
	;
ELSIF (v_table = 'VOC_Term')
THEN
	OPEN seq_cursor FOR
	SELECT _Vocab_key, sequenceNum
	FROM VOC_Term
	WHERE _Vocab_key = v_key
	ORDER BY sequenceNum
	;
ELSIF (v_table = 'MGI_SetMember')
THEN
	OPEN seq_cursor FOR
	SELECT _SetMember_key, sequenceNum
	FROM MGI_SetMember
	WHERE _Set_key = v_key and _CreatedBy_key = v_userKey
	ORDER BY sequenceNum
	;
ELSIF (v_table = 'VOC_AnnotHeaderMP')
THEN
	OPEN seq_cursor FOR
	SELECT _Object_key, sequenceNum
	FROM VOC_AnnotHeader
	WHERE _AnnotType_key = 1002 and _Object_key = v_key
	ORDER BY sequenceNum
	;
else
	return;
END IF;
LOOP
        FETCH seq_cursor INTO pkey, oldSeq;
        EXIT WHEN NOT FOUND;
	IF (v_table = 'GXD_AllelePair')
	THEN
		UPDATE GXD_AllelePair set sequenceNum = newSeq
		WHERE _AllelePair_key = pkey;
	ELSIF (v_table = 'GXD_AlleleGenotype')
	THEN
		UPDATE GXD_AlleleGenotype set sequenceNum = newSeq
		WHERE _Allele_key = pkey;
	ELSIF (v_table = 'GXD_GelLane')
	THEN
		UPDATE GXD_GelLane set sequenceNum = newSeq
		WHERE _GelLane_key = pkey;
	ELSIF (v_table = 'GXD_GelRow')
	THEN
		UPDATE GXD_GelRow set sequenceNum = newSeq
		WHERE _GelRow_key = pkey;
	ELSIF (v_table = 'GXD_Specimen')
	THEN
		UPDATE GXD_Specimen set sequenceNum = newSeq
		WHERE _Specimen_key = pkey;
	ELSIF (v_table = 'GXD_InSituResult')
	THEN
		UPDATE GXD_InSituResult set sequenceNum = newSeq
		WHERE _Result_key = pkey;
	ELSIF (v_table = 'MRK_History')
	THEN
		UPDATE MRK_History set sequenceNum = newSeq
		WHERE _Marker_key = pkey and sequenceNum = oldSeq;
	ELSIF (v_table = 'MLD_Expt_Marker')
	THEN
		UPDATE MLD_Expt_Marker set sequenceNum = newSeq
		WHERE _Expt_key = pkey and sequenceNum = oldSeq;
	ELSIF (v_table = 'MLD_MCDataList')
	THEN
		UPDATE MLD_MCDataList set sequenceNum = newSeq
		WHERE _Expt_key = pkey and sequenceNum = oldSeq;
	ELSIF (v_table = 'MLD_MC2point')
	THEN
		UPDATE MLD_MC2point set sequenceNum = newSeq
		WHERE _Expt_key = pkey and sequenceNum = oldSeq;
	ELSIF (v_table = 'MLD_RIData')
	THEN
		UPDATE MLD_RIData set sequenceNum = newSeq
		WHERE _Expt_key = pkey and sequenceNum = oldSeq;
	ELSIF (v_table = 'MLD_RI2Point')
	THEN
		UPDATE MLD_RI2Point set sequenceNum = newSeq
		WHERE _Expt_key = pkey and sequenceNum = oldSeq;
	ELSIF (v_table = 'MLD_FISH_Region')
	THEN
		UPDATE MLD_FISH_Region set sequenceNum = newSeq
		WHERE _Expt_key = pkey and sequenceNum = oldSeq;
	ELSIF (v_table = 'MRK_Chromosome')
	THEN
		UPDATE MRK_Chromosome set sequenceNum = newSeq
		WHERE _Chromosome_key = pkey;
	ELSIF (v_table = 'MLD_Statistics')
	THEN
		UPDATE MLD_Statistics set sequenceNum = newSeq
		WHERE _Expt_key = pkey and sequenceNum = oldSeq;
	ELSIF (v_table = 'MGI_Translation')
	THEN
		UPDATE MGI_Translation set sequenceNum = newSeq
		WHERE _TranslationType_key = pkey and sequenceNum = oldSeq;
	ELSIF (v_table = 'VOC_Term')
	THEN
		UPDATE VOC_Term set sequenceNum = newSeq
		WHERE _Vocab_key = pkey and sequenceNum = oldSeq;
	ELSIF (v_table = 'VOC_AnnotHeaderMP')
	THEN
		UPDATE VOC_AnnotHeader set sequenceNum = newSeq
		WHERE _AnnotType_key = 1002 and _Object_key = pkey and sequenceNum = oldSeq;
	ELSIF (v_table = 'MGI_SetMember')
	THEN
		UPDATE MGI_SetMember set sequenceNum = newSeq
		WHERE _SetMember_key = pkey and _CreatedBy_key = v_userKey and sequenceNum = oldSeq;
	END IF;
        newSeq := newSeq + 1;
END LOOP;
CLOSE seq_cursor;
RETURN;
END;
$$;


CREATE FUNCTION mgd.mgi_setmember_emapa_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: MGI_SetMember_EMAPA_insert()
-- DESCRIPTOIN:
--	this insert trigger will call MGI_checkEMAPAClipboard
--		and throw an Exception if the NEW
--		record is invalid
-- INPUT:
--	none
-- RETURNS:
--	NEW
PERFORM MGI_checkEMAPAClipboard(NEW._setmember_key);
PERFORM MGI_deleteEMAPAClipboardDups(NEW._setmember_key);
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.mgi_setmember_emapa_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: MGI_SetMember_EMAPA_update()
-- DESCRIPTOIN:
--	this update trigger will call MGI_checkEMAPAClipboard
--		and throw an Exception if the NEW
--		record is invalid
-- INPUT:
--	none
-- RETURNS:
--	NEW
PERFORM MGI_checkEMAPAClipboard(NEW._setmember_key);
PERFORM MGI_deleteEMAPAClipboardDups(NEW._setmember_key);
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.mgi_statistic_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: MGI_Statistic_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INPUT:
--	none
-- RETURNS:
--	NEW
DELETE FROM MGI_SetMember msm
USING MGI_Set ms
WHERE msm._Object_key = OLD._Statistic_key
AND msm._Set_key = ms._Set_key
AND ms._MGIType_key = 34
;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.mgi_updatereferenceassoc(v_userkey integer, v_mgitypekey integer, v_objectkey integer, v_refskey integer, v_refstypekey integer, v_assockey integer DEFAULT NULL::integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
 
DECLARE
v_allelekey int;
BEGIN
-- for genotypes
IF v_mgiTypeKey = 12
THEN
	FOR v_allelekey IN
	SELECT _Allele_key FROM GXD_AlleleGenotype WHERE _Genotype_key = v_objectKey
	LOOP
		-- Update non-assocType (see query below)
		-- Reference to new Reference Type 
		-- If no such record exists, then create a new Allele Reference record
		-- for each Allele of the Gentoype
		IF EXISTS (SELECT 1 FROM MGI_Reference_Assoc r, MGI_RefAssocType rt
        		WHERE r._Object_key = v_allelekey
			AND r._MGIType_key = 11
        		AND r._Refs_key = v_refsKey
			AND r._RefAssocType_key = rt._RefAssocType_key
        		AND rt.assocType not in ('Original', 
				'Molecular', 'Transmission', 
				'Mixed', 'Sequence', 'Treatment'))
		THEN
    			UPDATE MGI_Reference_Assoc
    			SET _RefAssocType_key = v_refsTypeKey
    			FROM MGI_RefAssocType rt
    			WHERE MGI_Reference_Assoc._Object_key = v_allelekey
			AND MGI_Reference_Assoc._MGIType_key = 11
        		AND MGI_Reference_Assoc._Refs_key = v_refsKey
			AND MGI_Reference_Assoc._RefAssocType_key = rt._RefAssocType_key
        		AND rt.assocType not in ('Original', 
				'Molecular', 'Transmission', 
				'Mixed', 'Sequence', 'Treatment')
			;
		ELSE
  			PERFORM MGI_insertReferenceAssoc (v_userKey, 11, v_allelekey, v_refsKey, v_refsTypeKey);
		END IF;
	END LOOP;
ELSEIF v_assocKey is not null
THEN
	UPDATE MGI_Reference_Assoc
	SET _Object_key = v_objectKey, 
                _RefAssocType_key = v_refsTypeKey, 
                _Refs_key = v_refsKey,
                _ModifiedBy_key = v_userKey, modification_date = now()
	WHERE MGI_Reference_Assoc._Assoc_key = v_assocKey;
END IF;
RETURN;
END;
$$;


CREATE FUNCTION mgd.mgi_updatesetmember(v_setmemberkey integer, v_sequencenum integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: MGI_updateSetMember
-- DESCRIPTION:
-- To modify the MGI_SetMember.sequenceNum
--        
-- INPUT:
--      
-- v_setMemberKey   : MGI_SetMember._SetMember_key
-- v_sequenceNum    : MGI_SetMember.sequenceNum
-- RETURNS:
--	VOID
UPDATE MGI_SetMember
SET sequenceNum = v_sequenceNum, modification_date = now()
WHERE _SetMember_key = v_setMemberKey
;
RETURN;
END;
$$;


CREATE FUNCTION mgd.mld_expt_marker_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- Propagate modification of Marker to experiment tables
UPDATE MLD_Concordance 
SET _Marker_key = NEW._Marker_key
WHERE MLD_Concordance._Expt_key = NEW._Expt_key
AND MLD_Concordance._Marker_key = OLD._Marker_key
;
UPDATE MLD_MC2point 
SET _Marker_key_1 = NEW._Marker_key
WHERE MLD_MC2point._Expt_key = NEW._Expt_key
AND MLD_MC2point._Marker_key_1 = OLD._Marker_key
;
UPDATE MLD_MC2point 
SET _Marker_key_2 = NEW._Marker_key
WHERE MLD_MC2point._Expt_key = NEW._Expt_key
AND MLD_MC2point._Marker_key_2 = OLD._Marker_key
;
UPDATE MLD_RIData 
SET _Marker_key = NEW._Marker_key
WHERE MLD_RIData._Expt_key = NEW._Expt_key
AND MLD_RIData._Marker_key = OLD._Marker_key
;
UPDATE MLD_RI2Point 
SET _Marker_key_1 = NEW._Marker_key
WHERE MLD_RI2Point._Expt_key = NEW._Expt_key
AND MLD_RI2Point._Marker_key_1 = OLD._Marker_key
;
UPDATE MLD_RI2Point 
SET _Marker_key_2 = NEW._Marker_key
WHERE MLD_RI2Point._Expt_key = NEW._Expt_key
AND MLD_RI2Point._Marker_key_2 = OLD._Marker_key
;
UPDATE MLD_Statistics
SET _Marker_key_1 = NEW._Marker_key
WHERE MLD_Statistics._Expt_key = NEW._Expt_key
AND MLD_Statistics._Marker_key_1 = OLD._Marker_key
;
UPDATE MLD_Statistics
SET _Marker_key_2 = NEW._Marker_key
WHERE MLD_Statistics._Expt_key = NEW._Expt_key
AND MLD_Statistics._Marker_key_2 = OLD._Marker_key
;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.mld_expts_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: MLD_Expts_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INPUT:
--	none
-- RETURNS:
--	NEW
-- Re-order the tag numbers for experiments if one is deleted
UPDATE MLD_Expts
SET tag = tag - 1
WHERE _Refs_key = OLD._Refs_key
AND exptType = OLD.exptType
AND tag > OLD.tag
;
DELETE FROM IMG_ImagePane_Assoc a
WHERE a._Object_key = OLD._Expt_key
AND a._MGIType_key = 4
;
DELETE FROM MAP_Coord_Feature a
WHERE a._Object_key = OLD._Expt_key
AND a._MGIType_key = 4
;
DELETE FROM MAP_Coordinate a
WHERE a._Object_key = OLD._Expt_key
AND a._MGIType_key = 4
;
DELETE FROM MGI_Note a
WHERE a._Object_key = OLD._Expt_key
AND a._MGIType_key = 4
;
DELETE FROM MGI_Property a
WHERE a._Object_key = OLD._Expt_key
AND a._MGIType_key = 4
;
DELETE FROM MGI_Reference_Assoc a
WHERE a._Object_key = OLD._Expt_key
AND a._MGIType_key = 4
;
DELETE FROM MGI_Synonym a
WHERE a._Object_key = OLD._Expt_key
AND a._MGIType_key = 4
;
DELETE FROM ACC_Accession a
WHERE a._Object_key = OLD._Expt_key
AND a._MGIType_key = 4
;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.mld_expts_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: MLD_Expts_insert()
-- DESCRIPTOIN:
--      1) if the Reference is for group = 'QTL'
--              in BIB_Workflow_Status, and the status is not 'Full-coded'
--         then
--              set existing BIB_Workflow_Status.isCurrent = 0
--              add new BIB_Workflow_Status._Status_key = 'Full-coded'
--	2) this insert trigger will call ACC_assignMGI
--	   in order to add a distinct MGI accession id
--	   to the NEW object
-- changes to this trigger require changes to procedure/BIB_updateWFStatusQTL_create.object
-- INPUT:
--	none
-- RETURNS:
--	NEW
IF EXISTS (SELECT 1 FROM BIB_Workflow_Status b
        WHERE NEW._Refs_key = b._Refs_key
        AND b._Group_key = 31576668
        AND b._Status_key != 31576674
	AND b.isCurrent = 1
	AND NEW.expttype in ('TEXT-QTL', 'TEXT-QTL-Candidate Genes', 'TEXT-Congenic', 'TEXT-Meta Analysis'))
THEN
    UPDATE BIB_Workflow_Status w set isCurrent = 0
    WHERE NEW._Refs_key = w._Refs_key
        AND w._Group_key = 31576668
    ;
    INSERT INTO BIB_Workflow_Status
    VALUES((select nextval('bib_workflow_status_seq')), NEW._Refs_key, 31576668, 31576674, 1, 1001, 1001, now(), now())
    ;
    PERFORM BIB_keepWFRelevance (NEW._Refs_key, 1001);
END IF;
PERFORM ACC_assignMGI(1001, NEW._Expt_key, 'Experiment');
RETURN NEW;
END;
$$;


-- CREATE FUNCTION mgd.mrk_allelewithdrawal(v_userkey integer, v_oldkey integer, v_newkey integer, v_refskey integer, v_eventreasonkey integer, v_addassynonym integer DEFAULT 1) RETURNS void
--     LANGUAGE plpgsql
--     AS $$
-- DECLARE
-- v_oldSymbol mrk_marker.symbol%TYPE;
-- v_oldName mrk_marker.name%TYPE;
-- v_newSymbol  mrk_marker.symbol%TYPE;
-- BEGIN
-- -- This procedure will process an allele marker withdrawal.
-- -- An allele marker withdrawal requires:
-- --	a) the "old" marker key
-- --	b) the "new" marker key
-- --	c) the reference key
-- --	d) the event reason key
-- v_oldSymbol := symbol
-- FROM MRK_Marker 
-- WHERE _Marker_key = v_oldKey
--      AND _Organism_key = 1
--      AND _Marker_Status_key = 1
-- ;
-- v_oldName := name
-- FROM MRK_Marker 
-- WHERE _Marker_key = v_oldKey
--      AND _Organism_key = 1
--      AND _Marker_Status_key = 1
-- ;
-- v_newSymbol := symbol
-- FROM MRK_Marker 
-- WHERE _Marker_key = v_newKey
--      AND _Organism_key = 1
--      AND _Marker_Status_key = 1
-- ;
-- IF v_oldSymbol IS NULL
-- THEN
--         RAISE EXCEPTION E'\nMRK_alleleWithdrawal : Invalid Old Symbol Key %', v_oldKey;
--         RETURN;
-- END IF;
-- IF v_newSymbol IS NULL
-- THEN
--         RAISE EXCEPTION E'\nMRK_alleleWithdrawal : Invalid New Symbol Key %', v_newKey;
--         RETURN;
-- END IF;
-- PERFORM MRK_mergeWithdrawal (v_userKey, v_oldKey, v_newKey, v_refsKey, 106563607, v_eventReasonKey, v_addAsSynonym);
-- IF NOT FOUND
-- THEN
--         RAISE EXCEPTION E'\nMRK_alleleWithdrawal : Could not execute allele withdrawal';
--         RETURN;
-- END IF;
-- RETURN;
-- END;
-- $$;


CREATE FUNCTION mgd.mrk_cluster_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: MRK_Cluster_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INPUT:
--	none
-- RETURNS:
--	NEW
DELETE FROM IMG_ImagePane_Assoc a
WHERE a._Object_key = OLD._Cluster_key
AND a._MGIType_key = 39
;
DELETE FROM MAP_Coord_Feature a
WHERE a._Object_key = OLD._Cluster_key
AND a._MGIType_key = 39
;
DELETE FROM MAP_Coordinate a
WHERE a._Object_key = OLD._Cluster_key
AND a._MGIType_key = 39
;
DELETE FROM MGI_Note a
WHERE a._Object_key = OLD._Cluster_key
AND a._MGIType_key = 39
;
DELETE FROM MGI_Property a
WHERE a._Object_key = OLD._Cluster_key
AND a._MGIType_key = 39
;
DELETE FROM MGI_Reference_Assoc a
WHERE a._Object_key = OLD._Cluster_key
AND a._MGIType_key = 39
;
DELETE FROM MGI_Synonym a
WHERE a._Object_key = OLD._Cluster_key
AND a._MGIType_key = 39
;
DELETE FROM ACC_Accession a
WHERE a._Object_key = OLD._Cluster_key
AND a._MGIType_key = 39
;
RETURN NEW;
END;
$$;


-- CREATE FUNCTION mgd.mrk_copyhistory(v_userkey integer, v_oldkey integer, v_newkey integer) RETURNS void
--     LANGUAGE plpgsql
--     AS $$
-- DECLARE
-- v_historyKey int;
-- v_refsKey int;
-- v_eventKey int;
-- v_eventReasonKey int;
-- v_createdByKey int;
-- v_modifiedByKey int;
-- v_name mrk_history.name%TYPE;
-- v_event_date mrk_history.event_date%TYPE;
-- BEGIN
-- -- Copy History of v_oldKey to v_newKey
-- FOR v_historyKey, v_refsKey, v_eventKey, v_eventReasonKey, v_name, v_event_date IN
-- SELECT _History_key, _Refs_key, _Marker_Event_key, _Marker_EventReason_key, name, event_date
-- FROM MRK_History
-- WHERE _Marker_key = v_oldKey
-- ORDER BY sequenceNum
-- LOOP
-- 	PERFORM MRK_insertHistory (v_userKey, v_newKey, v_historyKey, v_refsKey, v_eventKey, 
-- 		v_eventReasonKey, v_name, v_event_date);
-- END LOOP;
-- END;
-- $$;


-- CREATE FUNCTION mgd.mrk_deletewithdrawal(v_userkey integer, v_oldkey integer, v_refskey integer, v_eventreasonkey integer) RETURNS void
--     LANGUAGE plpgsql
--     AS $$
-- DECLARE
-- v_oldName mrk_marker.name%TYPE;
-- BEGIN
-- -- This procedure will process a delete withdrawal.
-- -- A delete marker withdrawal requires:
-- --	a) the "old" marker key
-- --	b) a reference key
-- IF EXISTS (SELECT 1 FROM ALL_Allele WHERE _Marker_key = v_oldKey and isWildType = 0)
-- THEN
--         RAISE EXCEPTION E'\nMRK_deleteWithdrawal: Cannot Delete:  Marker is referenced by Allele.';
--         RETURN;
-- END IF;
-- v_oldName := (SELECT name FROM MRK_Marker
-- 	 WHERE _Organism_key = 1
-- 	 AND _Marker_Status_key = 1
-- 	 AND _Marker_key = v_oldKey);
-- -- Update Marker info
-- UPDATE MRK_Marker 
-- SET name = 'withdrawn', _Marker_Status_key = 2 , 
-- 	cmOffset = -999.0,
-- 	_ModifiedBy_key = v_userKey, modification_date = now() 
-- WHERE _Marker_key = v_oldKey
-- ;
-- IF NOT FOUND
-- THEN
-- 	RAISE EXCEPTION E'MRK_deleteWithdrawal: Could not update marker';
-- 	RETURN;
-- END IF;
-- -- Add History line for withdrawal
-- PERFORM MRK_insertHistory (v_userKey, v_oldKey, v_oldKey, v_refsKey, 106563609, v_eventReasonKey, v_oldName);
-- IF NOT FOUND
-- THEN
-- 	RAISE EXCEPTION E'MRK_deleteWithdrawal: Could not add history';
-- 	RETURN;
-- END IF;
-- -- Remove old symbol's wild type allele
-- DELETE FROM ALL_Allele a
-- USING MRK_Marker m
-- WHERE m._Marker_key = v_oldKey
-- AND m._Marker_key = a._Marker_key
-- AND a.isWildType = 1 
-- ;
-- -- Remove MGI_Relationships that are annotated to this Marker
-- CREATE TEMP TABLE toDelete ON COMMIT DROP
-- AS SELECT r._Relationship_key
-- FROM MGI_Relationship r, MGI_Relationship_Category c
-- WHERE r._Object_key_1 = v_oldKey
-- AND r._Category_key  = c._Category_key
-- AND c._MGIType_key_1 = 2
-- UNION
-- SELECT r._Relationship_key
-- FROM MGI_Relationship r, MGI_Relationship_Category c
-- WHERE r._Object_key_2 = v_oldKey
-- AND r._Category_key  = c._Category_key
-- AND c._MGIType_key_2 = 2
-- ;
-- DELETE 
-- FROM MGI_Relationship
-- USING toDelete d
-- WHERE d._Relationship_key = MGI_Relationship._Relationship_key
-- ;
-- RETURN;
-- END;
-- $$;


CREATE FUNCTION mgd.mrk_inserthistory(userkey integer, markerkey bigint, historykey bigint, refkey integer, eventkey integer, eventreasonkey integer DEFAULT 106563610, name text DEFAULT NULL::text, event_date timestamp without time zone DEFAULT NULL::timestamp without time zone, createdbykey integer DEFAULT NULL::integer, modifiedbykey integer DEFAULT NULL::integer, creation_date timestamp without time zone DEFAULT NULL::timestamp without time zone, modification_date timestamp without time zone DEFAULT NULL::timestamp without time zone) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
maxSeq int;
BEGIN
-- Insert new History record into MRK_History
maxSeq := max(sequenceNum) from MRK_History where _Marker_key = markerKey;
IF maxSeq IS NULL
THEN
	maxSeq := 0;
END IF;
IF event_date IS NULL
THEN
	event_date := now();
END IF;
IF createdByKey IS NULL
THEN
	createdByKey := userKey;
END IF;
IF modifiedByKey IS NULL
THEN
	modifiedByKey := userKey;
END IF;
IF creation_date IS NULL
THEN
	creation_date := now();
END IF;
IF modification_date IS NULL
THEN
	modification_date := now();
END IF;
INSERT INTO MRK_History
(_Assoc_key, _Marker_key, _History_key, _Refs_key, _Marker_Event_key, _Marker_EventReason_key, sequenceNum, 
name, event_date, _CreatedBy_key, _ModifiedBy_key, creation_date, modification_date)
VALUES((select nextval('mrk_history_seq')), markerKey, historyKey, refKey, eventKey, eventReasonKey, maxSeq + 1, name, 
       event_date, createdByKey, modifiedByKey, creation_date, modification_date)
;
END;
$$;


CREATE FUNCTION mgd.mrk_marker_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: MRK_Marker_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INPUT:
--	none
-- RETURNS:
--	NEW
-- Disallow deletion if Marker is currently referenced elsewhere
IF EXISTS (SELECT * FROM MGI_Relationship
          WHERE OLD._Marker_key = _Object_key_1
          AND _Category_key in (1001, 1002, 1008))
THEN
        RAISE EXCEPTION E'Marker Symbol is referenced in MGI Relationship(s)';
END IF;
-- Disallow deletion if Marker is currently referenced elsewhere
IF EXISTS (SELECT * FROM MGI_Relationship
          WHERE OLD._Marker_key = _Object_key_2
          AND _Category_key in (1001, 1002, 1003, 1004, 1006, 1008))
THEN
        RAISE EXCEPTION E'Marker Symbol is referenced in MGI Relationship(s)';
END IF;
IF EXISTS (SELECT * FROM VOC_Annot a
          WHERE OLD._Marker_key = a._Object_key
          AND a._AnnotType_key = 1000)
THEN
        RAISE EXCEPTION E'Marker is referenced in GO-Marker Annotations';
END IF; 
IF EXISTS (SELECT * FROM VOC_Annot a
          WHERE OLD._Marker_key = a._Object_key
          AND a._AnnotType_key = 1003)
THEN
        RAISE EXCEPTION E'Marker is referenced in InterPro-Marker Annotations';
END IF; 
IF EXISTS (SELECT * FROM VOC_Annot a
          WHERE OLD._Marker_key = a._Object_key
          AND a._AnnotType_key = 1007)
THEN
        RAISE EXCEPTION E'Marker is referenced in PIRSF-Marker Annotations';
END IF; 
--a cascading delete will remove the mcv annotation
--IF EXISTS (SELECT * FROM VOC_Annot a
--          WHERE OLD._Marker_key = a._Object_key
--          AND a._AnnotType_key = 1011)
--THEN
--        RAISE EXCEPTION E'Marker is referenced in MCV-Marker Annotations';
--END IF; 
IF EXISTS (SELECT * FROM VOC_Annot a
          WHERE OLD._Marker_key = a._Object_key
          AND a._AnnotType_key = 1015)
THEN
        RAISE EXCEPTION E'Marker is referenced in Mammalian Phenotype-Marker Annotations %', OLD._Marker_key;
END IF; 
IF EXISTS (SELECT * FROM VOC_Annot a
          WHERE OLD._Marker_key = a._Object_key
          AND a._AnnotType_key = 1022)
THEN
        RAISE EXCEPTION E'Marker is referenced in DO-Human Marker Annotations';
END IF; 
IF EXISTS (SELECT * FROM VOC_Annot a
          WHERE OLD._Marker_key = a._Object_key
          AND a._AnnotType_key = 1023)
THEN
        RAISE EXCEPTION E'Marker is referenced in DO/Marker Annotations';
END IF; 
IF EXISTS (SELECT * FROM VOC_Annot a
          WHERE OLD._Marker_key = a._Object_key
          AND a._AnnotType_key = 1017)
THEN
        RAISE EXCEPTION E'Marker is referenced in Protein Ontology/Marker Annotations';
END IF; 
DELETE FROM IMG_ImagePane_Assoc a
WHERE a._Object_key = OLD._Marker_key
AND a._MGIType_key = 2
;
DELETE FROM MAP_Coord_Feature a
WHERE a._Object_key = OLD._Marker_key
AND a._MGIType_key = 2
;
DELETE FROM MAP_Coordinate a
WHERE a._Object_key = OLD._Marker_key
AND a._MGIType_key = 2
;
DELETE FROM MGI_Note a
WHERE a._Object_key = OLD._Marker_key
AND a._MGIType_key = 2
;
DELETE FROM MGI_Property a
WHERE a._Object_key = OLD._Marker_key
AND a._MGIType_key = 2
;
DELETE FROM MGI_Reference_Assoc a
WHERE a._Object_key = OLD._Marker_key
AND a._MGIType_key = 2
;
DELETE FROM MGI_Synonym a
WHERE a._Object_key = OLD._Marker_key
AND a._MGIType_key = 2
;
DELETE FROM VOC_Annot a
WHERE a._Object_key = OLD._Marker_key
AND a._AnnotType_key = 1011
;
DELETE FROM ACC_Accession a
WHERE a._Object_key = OLD._Marker_key
AND a._MGIType_key = 2
;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.mrk_marker_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
v_offset float;
BEGIN
-- NAME: MRK_Marker_insert()
-- DESCRIPTOIN:
--      this insert trigger will call ACC_assignMGI
--      in order to add a distinct MGI accession id
--      to the NEW object
-- INPUT:
--      none
-- RETURNS:
--      NEW 
-- Only Mouse Markers use the rest of this trigger
IF (NEW._Organism_key != 1)
THEN
    RETURN NEW;
END IF;
-- No duplicates
IF (SELECT count(*) FROM MRK_Marker WHERE _Organism_key = 1 AND _Marker_Status_key in (1,3) AND symbol = NEW.symbol group by symbol, chromosome having count(*) > 1)
THEN
    RAISE EXCEPTION E'Symbol "%" already exists\n', NEW.symbol;
    RETURN NEW;
END IF;
-- Create Current for new marker
INSERT INTO MRK_Current VALUES(New._Marker_key, NEW._Marker_key, now(), now());
IF NOT FOUND
THEN
    RAISE EXCEPTION E'MRK_Current is not found';
    RETURN NEW;
END IF;
-- Create Offset for new marker
--IF (NEW.chromosome = 'UN')
--THEN
        --v_offset := -999.0;
--ELSE
        --v_offset := -1.0;
--END IF; 
-- this is called from the EI/Marker module due to Reference rules
-- Create History line for new marker
-- J:23000 : 22864
--PERFORM MRK_insertHistory (NEW._CreatedBy_key, NEW._Marker_key, NEW._Marker_key, 22864, 1, -1,  NEW.name);
--IF NOT FOUND
--THEN
    --RAISE EXCEPTION E'MRK_insertHistory is not found';
    --RETURN NEW;
--END IF;
-- If marker is 'official', and 'gene', etc., then create wild type Allele
IF NEW._Marker_Status_key in (1)
   AND NEW._Marker_Type_key = 1 
   AND NEW.symbol not like 'mt-%'
   AND NEW.name not like 'withdrawn, =%' 
   AND NEW.name not like '%dna segment%'
   AND NEW.name not like 'EST %'
   AND NEW.name not like '%expressed sequence%'
   AND NEW.name not like '%cDNA sequence%'
   AND NEW.name not like '%gene model%'
   AND NEW.name not like '%hypothetical protein%'
   AND NEW.name not like '%ecotropic viral integration site%'        
   AND NEW.name not like '%viral polymerase%'
   AND NOT EXISTS (SELECT 1 FROM ALL_Allele where _Marker_key = NEW._Marker_key and isWildType = 1)
THEN
        PERFORM ALL_createWildType (NEW._ModifiedBy_key, NEW._Marker_key, NEW.symbol);
        IF NOT FOUND
        THEN
                RAISE EXCEPTION E'ALL_createWildType';
                RETURN NEW;
        END IF;
END IF;
-- Create Accession id for new marker
PERFORM ACC_assignMGI(1001, NEW._Marker_key, 'Marker');
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.mrk_marker_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- Only Mouse Markers use this trigger
IF (NEW._Organism_key != 1)
THEN
    RETURN NEW;
END IF;
-- No duplicates
IF (SELECT count(*) FROM MRK_Marker WHERE _Organism_key = 1 AND _Marker_Status_key in (1,3) AND symbol = NEW.symbol group by symbol, chromosome having count(*) > 1)
THEN
    RAISE EXCEPTION E'Symbol "%" already exists', NEW.symbol;
    RETURN NEW;
END IF;
-- from: 'reserved' (_Marker_Status_key = 3)
-- to:  'official' (_Marker_Status_key = 1)
-- and:
--	reference = J:23000
IF OLD._Marker_Status_key in (3)
   AND NEW._Marker_Status_key in (1)
   AND EXISTS (SELECT 1 FROM MRK_History 
	WHERE _Marker_key = NEW._Marker_key 
	AND _History_key = NEW._Marker_key
	AND _Marker_Event_key = 1
	AND sequenceNum = 1
	AND _Refs_key = 22864
	)
THEN
	RAISE NOTICE E'Symbol is using J:23000';
END IF;
-- and:
--      marker type = 'Gene'
--      and wild-type allele does not exist
--      see other conitionals below
-- then create wild-type allele
IF OLD._Marker_Status_key in (3)
   AND NEW._Marker_Status_key in (1)
   AND NEW._Marker_Type_key = 1
   AND NEW.symbol not like 'mt-%'
   AND NEW.name not like 'withdrawn, =%'
   AND NEW.name not like '%dna segment%'
   AND NEW.name not like 'EST %'
   AND NEW.name not like '%expressed sequence%'
   AND NEW.name not like '%cDNA sequence%'
   AND NEW.name not like '%gene model%'
   AND NEW.name not like '%hypothetical protein%'
   AND NEW.name not like '%ecotropic viral integration site%'
   AND NEW.name not like '%viral polymerase%'
   AND NOT EXISTS (SELECT 1 FROM ALL_Allele where _Marker_key = NEW._Marker_key and isWildType = 1)
THEN
        PERFORM ALL_createWildType (NEW._ModifiedBy_key, NEW._Marker_key, NEW.symbol);
        IF NOT FOUND
        THEN
                RAISE EXCEPTION E'ALL_createWildType';
                RETURN NEW;
        END IF;
END IF;
IF OLD._Marker_Status_key in (1) AND NEW._Marker_Status_key in (3)
	AND (EXISTS (SELECT 1 FROM ALL_Allele WHERE NEW._Marker_key = _Marker_key)
	     OR EXISTS (SELECT 1 FROM ALL_Knockout_Cache WHERE NEW._Marker_key = _Marker_key)
	     OR EXISTS (SELECT 1 FROM GO_Tracking WHERE NEW._Marker_key = _Marker_key)
	     OR EXISTS (SELECT 1 FROM GXD_AlleleGenotype WHERE NEW._Marker_key = _Marker_key)
	     OR EXISTS (SELECT 1 FROM GXD_AllelePair WHERE NEW._Marker_key = _Marker_key)
	     OR EXISTS (SELECT 1 FROM GXD_AntibodyMarker WHERE NEW._Marker_key = _Marker_key)
	     OR EXISTS (SELECT 1 FROM GXD_Assay WHERE NEW._Marker_key = _Marker_key)
	     OR EXISTS (SELECT 1 FROM GXD_Expression WHERE NEW._Marker_key = _Marker_key)
	     OR EXISTS (SELECT 1 FROM GXD_Index WHERE NEW._Marker_key = _Marker_key)
	     OR EXISTS (SELECT 1 FROM MLD_Expt_Marker WHERE NEW._Marker_key = _Marker_key)
	     OR EXISTS (SELECT 1 FROM MRK_ClusterMember WHERE NEW._Marker_key = _Marker_key)
	     OR EXISTS (SELECT 1 FROM MRK_MCV_Cache WHERE NEW._Marker_key = _Marker_key)
	     OR EXISTS (SELECT 1 FROM MRK_DO_Cache WHERE NEW._Marker_key = _Marker_key)
	     OR EXISTS (SELECT 1 FROM MRK_DO_Cache WHERE NEW._Marker_key = _Marker_key)
	     OR EXISTS (SELECT 1 FROM PRB_Marker WHERE NEW._Marker_key = _Marker_key)
	     OR EXISTS (SELECT 1 FROM PRB_RFLV WHERE NEW._Marker_key = _Marker_key)
	     OR EXISTS (SELECT 1 FROM PRB_Strain_Marker WHERE NEW._Marker_key = _Marker_key)
	     OR EXISTS (SELECT 1 FROM SEQ_Marker_Cache WHERE NEW._Marker_key = _Marker_key)
	     OR EXISTS (SELECT 1 FROM VOC_Marker_Cache WHERE NEW._Marker_key = _Marker_key)
	     OR EXISTS (SELECT 1 FROM WKS_Rosetta WHERE NEW._Marker_key = _Marker_key)
	     )
THEN
	RAISE EXCEPTION E'Cannot change "official" to "reserved" because Marker has annotations';
	RETURN NEW;
END IF;
-- TR11855 : 08/16/2018 lec
-- if chromosome value has changed, then add to "Needs Review - Chr"
IF OLD.chromosome != NEW.chromosome
THEN
	PERFORM PRB_setStrainReview (NEW._Marker_key, NULL, (select _Term_key from VOC_Term where _Vocab_key = 56 and term = 'Needs Review - chr'));
        IF NOT FOUND
        THEN
                RAISE EXCEPTION E'PRB_setStrainReview';
                RETURN NEW;
        END IF;
END IF;
RETURN NEW;
END;
$$;


-- CREATE FUNCTION mgd.mrk_mergewithdrawal(v_userkey integer, v_oldkey integer, v_newkey integer, v_refskey integer, v_eventkey integer, v_eventreasonkey integer, v_addassynonym integer DEFAULT 1) RETURNS void
--     LANGUAGE plpgsql
--     AS $$
-- DECLARE
-- v_alleleOf int;
-- v_assigningRefKey int;
-- v_synTypeKey int;
-- v_oldSymbol mrk_marker.symbol%TYPE;
-- v_oldName mrk_marker.name%TYPE;
-- v_newSymbol mrk_marker.symbol%TYPE;
-- v_newChr mrk_marker.chromosome%TYPE;
-- v_withdrawnName mrk_marker.name%TYPE;
-- v_cytogeneticOffset mrk_marker.cytogeneticOffset%TYPE;
-- v_cmOffset mrk_marker.cmOffset%TYPE;
-- v_alleleSymbol all_allele.symbol%TYPE;
-- BEGIN
-- -- This procedure will process a merge marker withdrawal.
-- -- A merge withdrawal is a withdrawal where both the "old" and "new"
-- -- markers already exist in the database.
-- -- A merge marker withdrawal requires:
-- --	a) the "old" marker key
-- --	b) the "new" marker key
-- --	c) the reference key
-- --	d) the event key
-- --	e) the event reason key
-- --	f) the "add as synonym" flag
-- IF v_oldKey = v_newKey
-- THEN
-- 	RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Cannot Merge a Symbol into itself: %, %', v_oldKey, v_newKey;
-- 	RETURN;
-- END IF;
-- v_oldSymbol := symbol
-- 	FROM MRK_Marker 
-- 	WHERE _Marker_key = v_oldKey
--      	AND _Organism_key = 1
--      	AND _Marker_Status_key = 1;
-- v_oldName := name
-- 	FROM MRK_Marker 
-- 	WHERE _Marker_key = v_oldKey
--      	AND _Organism_key = 1
--      	AND _Marker_Status_key = 1;
-- IF v_oldSymbol IS NULL
-- THEN
-- 	RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Invalid Old Symbol Key %', v_oldKey;
-- 	RETURN;
-- END IF;
-- v_newSymbol := (
-- 	SELECT symbol
-- 	FROM MRK_Marker 
-- 	WHERE _Marker_key = v_newKey
--      	AND _Organism_key = 1
--      	AND _Marker_Status_key = 1
-- 	);
-- IF v_newSymbol IS NULL
-- THEN
-- 	RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Invalid New Symbol Key %', v_newKey;
-- 	RETURN;
-- END IF;
-- -- Prevent the merge if the a Marker Detail Clip exists for both symbols
-- IF EXISTS (SELECT 1 FROM MRK_Notes WHERE _Marker_key = v_oldKey) AND
--    EXISTS (SELECT 1 FROM MRK_Notes WHERE _Marker_key = v_newKey)
-- THEN
--         RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Cannot Merge:  both Symbols contain a Marker Detail Clip.';
--         RETURN;
-- END IF;
-- -- Else, continue....
-- if v_eventKey = 4
-- THEN
-- 	v_withdrawnName := 'withdrawn, allele of ' || v_newSymbol;
-- 	v_alleleOf := 1;
-- ELSE
-- 	v_withdrawnName := 'withdrawn, = ' || v_newSymbol;
-- 	v_alleleOf := 0;
-- END IF;
-- --see TR11855 : make sure new term exists before turning this on
-- --if v_eventKey = 4
-- --THEN
--         -- Update needsReview flag for strains
--         --PERFORM PRB_setStrainReview (v_oldKey, NULL, (select _Term_key from VOC_Term where _Vocab_key = 56 and term = 'Needs Review - Chr'));
--         --IF NOT FOUND
--         --THEN
-- 	        --RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not flag Strain record for needing review';
-- 	        --RETURN;
--         --END IF;
-- --END IF;
-- -- If new symbol has a chromosome of UN, update the new symbol's chromosome value
-- -- with the old symbol chromosome value
-- IF (SELECT chromosome FROM MRK_Marker WHERE _Marker_key = v_newKey) = 'UN'
-- THEN
-- 	v_newChr := (SELECT chromosome FROM MRK_Marker WHERE _Marker_key = v_oldKey);
-- 	UPDATE MRK_Marker 
-- 	SET chromosome = v_newChr, _ModifiedBy_key = v_userKey, modification_date = now()
-- 	WHERE _Marker_key = v_newKey
-- 	;
-- 	IF NOT FOUND
-- 	THEN
-- 		RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not update new symbol/chromosome';
-- 		RETURN;
-- 	END IF;
-- END IF;
-- -- Update cytogenetic/offset values of new symbol
-- v_cytogeneticOffset := (SELECT cytogeneticOffset FROM MRK_Marker WHERE _Marker_key = v_oldKey);
-- v_cmOffset := (SELECT cmOffset FROM MRK_Marker WHERE _Marker_key = v_oldKey);
-- UPDATE MRK_Marker
-- SET cytogeneticOffset = v_cytogeneticOffset, cmOffset = v_cmOffset
-- WHERE _Marker_key = v_newKey
-- ;
-- IF NOT FOUND
-- THEN
-- 	RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not update cytogenetic/offset values';
-- 	RETURN;
-- END IF;
-- -- Update name/cytogenetic/offset of old symbol
-- UPDATE MRK_Marker 
-- SET name = v_withdrawnName, cytogeneticOffset = null, _Marker_Status_key = 2, cmOffset = -999.0,
-- 	_ModifiedBy_key = v_userKey, modification_date = now()
-- WHERE _Marker_key = v_oldKey
-- ;
-- IF NOT FOUND
-- THEN
-- 	RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not update name of old symbol : %', v_oldKey;
-- 	RETURN;
-- END IF;
-- -- Merge potential duplicate wild type alleles of old and new symbols
-- -- before converting oldsymbol alleles
-- PERFORM ALL_mergeWildTypes (v_oldKey, v_newKey, v_oldSymbol, v_newSymbol);
-- IF NOT FOUND
-- THEN
-- 	RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not merge wild type alleles';
-- 	RETURN;
-- END IF;
-- -- Convert Remaining Alleles
-- PERFORM ALL_convertAllele (v_userKey, v_oldKey, v_oldSymbol, v_newSymbol, v_alleleOf);
-- --IF NOT FOUND
-- --THEN
-- --	RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not convert alleles';
-- --	RETURN;
-- --END IF;
-- IF v_alleleOf = 1
-- THEN
-- 	-- If no alleles exist for the old symbol, create a newSymbol<oldSymbol> allele
-- 	IF NOT EXISTS (SELECT 1 FROM ALL_Allele WHERE _Marker_key = v_oldKey)
-- 	THEN
-- 		v_alleleSymbol := v_newSymbol || '<' || v_oldSymbol || '>';
-- 		PERFORM ALL_insertAllele (v_userKey, v_newKey, v_alleleSymbol, v_oldName, 
-- 			v_refsKey, 0, null, null, null, null,
-- 			-1, 'Not Specified', 'Not Specified', 'Approved', v_oldSymbol);
-- 		IF NOT FOUND
-- 		THEN
-- 			RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not insert allele';
-- 			RETURN;
-- 		END IF;
-- 	END IF;
-- END IF;
-- -- Merge Marker Feature Relationships (MGI_Relationship)
-- PERFORM MGI_mergeRelationship (v_oldKey, v_newKey);
-- IF NOT FOUND
-- THEN
-- 	RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not merge Marker Feature Relationships';
-- 	RETURN;
-- END IF;
-- -- Update current symbols
-- UPDATE MRK_Current 
-- SET _Current_key = v_newKey 
-- WHERE _Marker_key = v_oldKey
-- ;
-- IF NOT FOUND
-- THEN
-- 	RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not update current symbols';
-- 	RETURN;
-- END IF;
-- -- Copy History records from old symbol to new symbol
-- PERFORM MRK_copyHistory (v_userKey, v_oldKey, v_newKey);
-- IF NOT FOUND
-- THEN
-- 	RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not copy history records';
-- 	RETURN;
-- END IF;
-- -- Insert history record for withdrawal
-- PERFORM MRK_insertHistory (v_userKey, v_newKey, v_oldKey, v_refsKey, v_eventKey, v_eventReasonKey, v_oldName);
-- IF NOT FOUND
-- THEN
-- 	RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not create history record';
-- 	RETURN;
-- END IF;
-- -- Remove history records from old symbol
-- DELETE FROM MRK_History WHERE _Marker_key = v_oldKey;
-- -- Insert withdrawn symbol into Synonym table
-- -- Use assigning reference
-- IF v_addAsSynonym = 1
-- THEN
-- 	v_assigningRefKey := (
-- 		SELECT DISTINCT _Refs_key 
-- 		FROM MRK_History_View
-- 		WHERE _Marker_key = v_newKey
-- 		AND history = v_oldSymbol
-- 		AND _Marker_Event_key = 1
-- 		ORDER BY _Refs_key
-- 		LIMIT 1
-- 		);
-- 	-- If oldSymbol is a Riken symbol (ends with 'Rik'), 
-- 	-- then synonym type = 'similar' else synonym type = 'exact'
-- 	IF EXISTS (SELECT 1
-- 		   FROM MRK_Marker 
--                    WHERE _Marker_key = v_oldKey
--                         AND _Organism_key = 1
--                         AND _Marker_Status_key = 1
-- 		        AND symbol like '%Rik'
-- 		   )
-- 	THEN
-- 	    v_synTypeKey := 1005;
--         ELSE
-- 	    v_synTypeKey := 1004;
--         END IF;
-- 	PERFORM MGI_insertSynonym (v_userKey, v_newKey, 2, v_synTypeKey, v_oldSymbol, v_assigningRefKey, 1);
-- 	IF NOT FOUND
-- 	THEN
-- 		RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not add synonym';
-- 		RETURN;
-- 	END IF;
-- END IF;
-- -- Remove old symbol's wild type allele
-- DELETE FROM ALL_Allele a
-- USING MRK_Marker m
-- WHERE m._Marker_key = v_oldKey
-- AND m._Marker_key = a._Marker_key
-- AND a.isWildType = 1
-- ;
-- -- Update keys from old key to new key
-- PERFORM MRK_updateKeys (v_oldKey, v_newKey);
-- IF NOT FOUND
-- THEN
-- 	RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not update keys';
-- 	RETURN;
-- END IF;
-- END;
-- $$;


-- CREATE FUNCTION mgd.mrk_reloadlocation(v_markerkey integer) RETURNS void
--     LANGUAGE plpgsql
--     AS $$
-- DECLARE
-- v_markerTypeKey int;
-- v_organismKey int;
-- v_sequenceNum int;
-- v_chromosome mrk_marker.chromosome%TYPE;
-- v_cytogeneticOffset mrk_marker.cytogeneticOffset%TYPE;
-- v_cmoffset mrk_marker.cmoffset%TYPE;
-- v_startCoordinate seq_coord_cache.startCoordinate%TYPE;
-- v_endCoordinate seq_coord_cache.endCoordinate%TYPE;
-- v_strand seq_coord_cache.strand%TYPE;
-- v_mapUnits voc_term.term%TYPE;
-- v_provider map_coord_collection.abbreviation%TYPE;
-- v_version seq_coord_cache.version%TYPE;
-- v_genomicChromosome seq_coord_cache.chromosome%TYPE;
-- BEGIN
-- DELETE FROM MRK_Location_Cache where _Marker_key = v_markerKey;
-- FOR v_chromosome, v_cytogeneticOffset, v_markerTypeKey, v_organismKey, v_cmoffset, 
--     v_sequenceNum, v_startCoordinate, v_endCoordinate, v_strand, v_mapUnits, 
--     v_provider, v_version, v_genomicChromosome
-- IN
-- select m.chromosome, m.cytogeneticOffset, m._Marker_Type_key, m._Organism_key, m.cmOffset,
-- c.sequenceNum,
-- f.startCoordinate, f.endCoordinate, f.strand, u.term, cl.abbreviation, cc.version, gc.chromosome as genomicChromosome
-- from MRK_Marker m, MRK_Chromosome c,
-- MAP_Coord_Collection cl, MAP_Coordinate cc, MAP_Coord_Feature f, VOC_Term u, MRK_Chromosome gc
-- where m._Marker_key = v_markerKey
-- and m._Organism_key = c._Organism_key
-- and m.chromosome = c.chromosome
-- and m._Marker_key = f._Object_key
-- and f._MGIType_key = 2
-- and f._Map_key = cc._Map_key
-- and cc._Collection_key = cl._Collection_key
-- and cc._Units_key = u._Term_key
-- and cc._Object_key = gc._Chromosome_key
-- UNION
-- select m.chromosome, m.cytogeneticOffset, m._Marker_Type_key, m._Organism_key, m.cmOffset,
-- ch.sequenceNum,
-- c.startCoordinate, c.endCoordinate, c.strand, c.mapUnits, cl.abbreviation, c.version, c.chromosome as genomicChromosome
-- from MRK_Marker m, MRK_Chromosome ch, 
--      SEQ_Marker_Cache mc, SEQ_Coord_Cache c, MAP_Coordinate cc,
--      MAP_Coord_Collection cl
-- where m._Marker_key = v_markerKey
-- and m._Organism_key = ch._Organism_key 
-- and m.chromosome = ch.chromosome 
-- and m._Marker_key = mc._Marker_key
-- and mc._Qualifier_key = 615419
-- and mc._Sequence_key = c._Sequence_key
-- and c._Map_key = cc._Map_key
-- and cc._Collection_key = cl._Collection_key
-- UNION
-- select m.chromosome, m.cytogeneticOffset, m._Marker_Type_key, m._Organism_key, m.cmOffset,
-- c.sequenceNum,NULL,NULL,NULL,NULL,NULL,NULL,NULL
-- from MRK_Marker m, MRK_Chromosome c
-- where m._Marker_key = v_markerKey
-- and m._Organism_key = c._Organism_key
-- and m.chromosome = c.chromosome
-- and not exists (select 1 from SEQ_Marker_Cache mc, SEQ_Coord_Cache c
-- where m._Marker_key = mc._Marker_key
-- and mc._Qualifier_key = 615419
-- and mc._Sequence_key = c._Sequence_key)
-- and not exists (select 1 from MAP_Coord_Feature f
-- where m._Marker_key = f._Object_key
-- and f._MGIType_key = 2)
-- UNION
-- select m.chromosome, m.cytogeneticOffset, m._Marker_Type_key, m._Organism_key, m.cmOffset,
-- c.sequenceNum,NULL,NULL,NULL,NULL,NULL,NULL,NULL
-- from MRK_Marker m, MRK_Chromosome c
-- where m._Marker_key = v_markerKey
-- and m._Organism_key = c._Organism_key
-- and m.chromosome = c.chromosome
-- and not exists (select 1 from SEQ_Marker_Cache mc, SEQ_Coord_Cache c
-- where m._Marker_key = mc._Marker_key
-- and mc._Qualifier_key = 615419
-- and mc._Sequence_key = c._Sequence_key)
-- and not exists (select 1 from MAP_Coord_Feature f
-- where m._Marker_key = f._Object_key
-- and f._MGIType_key = 2)
-- LOOP
-- 	INSERT INTO MRK_Location_Cache
-- 	(_Marker_key, _Marker_Type_key, _Organism_key, chromosome, sequenceNum, cytogeneticOffset, 
-- 		cmoffset, genomicChromosome, startCoordinate, endCoordinate, strand, mapUnits, provider, version)
-- 	VALUES(v_markerKey, v_markerTypeKey, v_organismKey, v_chromosome, v_sequenceNum, v_cytogeneticOffset, 
-- 		v_cmoffset, v_genomicChromosome, v_startCoordinate, v_endCoordinate, v_strand, v_mapUnits, v_provider, v_version)
-- 	;
--         -- only process 1st value
--         EXIT;
-- END LOOP;
-- END;
-- $$;


CREATE FUNCTION mgd.mrk_reloadreference(v_markerkey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_refKey int;
BEGIN
-- Select all unique Marker/Reference pairs
DELETE FROM MRK_Reference WHERE _Marker_key = v_markerKey
;
FOR v_refKey IN
SELECT distinct m._Refs_key
FROM PRB_Marker m
WHERE m._Marker_key = v_markerKey
UNION 
SELECT distinct h._Refs_key
FROM MRK_History h
WHERE h._Marker_key = v_markerKey
AND h._Refs_key IS NOT NULL 
UNION 
SELECT distinct e._Refs_key
FROM MLD_Expts e, MLD_Expt_Marker em
WHERE em._Marker_key = v_markerKey
AND em._Expt_key = e._Expt_key
UNION 
SELECT distinct _Refs_key
FROM GXD_Index 
WHERE _Marker_key = v_markerKey
UNION 
SELECT distinct _Refs_key
FROM GXD_Assay 
WHERE _Marker_key = v_markerKey
UNION 
SELECT distinct _Refs_key
FROM MGI_Synonym
WHERE _Object_key = v_markerKey
AND _MGITYpe_key = 2
AND _Refs_key IS NOT NULL 
UNION 
SELECT distinct ar._Refs_key
FROM ACC_Accession a, ACC_AccessionReference ar 
WHERE a._Object_key = v_markerKey
AND a._MGIType_key = 2 
AND a.private = 0
AND a._Accession_key = ar._Accession_key 
UNION
SELECT distinct r._Refs_key
FROM ALL_Allele a, MGI_Reference_Assoc r
WHERE a._Marker_key = v_markerKey
AND a._Allele_key = r._Object_key
AND r._MGIType_key = 11
UNION
SELECT distinct r._Refs_key
FROM VOC_Annot a, VOC_Evidence r
WHERE a._AnnotType_key = 1000
AND a._Object_key = v_markerKey
AND a._Annot_key = r._Annot_key
UNION 
SELECT distinct _Refs_key
FROM MGI_Reference_Assoc
WHERE _Object_key = v_markerKey AND _MGIType_key = 2
LOOP
	IF NOT exists (SELECT * FROM MRK_Reference WHERE _Marker_key = v_markerKey
		AND _Refs_key = v_refKey)
	THEN
		INSERT INTO MRK_Reference (_Marker_key, _Refs_key, mgiID, jnumID, pubmedID, jnum)
		SELECT v_markerKey, v_refKey, a1.accID, a2.accID, a3.accID, a2.numericPart
		FROM ACC_Accession a1
			INNER JOIN ACC_Accession a2 on (
				a2._MGIType_key = 1
				AND a1._Object_key = a2._Object_key
				AND a2._LogicalDB_key = 1
				AND a2.prefixPart = 'J:'
				AND a2.preferred = 1)
			LEFT OUTER JOIN ACC_Accession a3 on (
				a1._Object_key = a3._Object_key
				AND a3._LogicalDB_key = 29
				AND a3.preferred = 1)
		WHERE a1._MGIType_key = 1
		AND a1._Object_key = v_refKey
		AND a1._LogicalDB_key = 1
		AND a1.prefixPart = 'MGI:'
		AND a1.preferred = 1
		;
	END IF;
END LOOP;
END;
$$;


-- CREATE FUNCTION mgd.mrk_simplewithdrawal(v_userkey integer, v_oldkey integer, v_refskey integer, v_eventreasonkey integer, v_newsymbol text, v_newname text, v_addassynonym integer DEFAULT 1) RETURNS void
--     LANGUAGE plpgsql
--     AS $$
-- DECLARE
-- v_newKey int;
-- v_withdrawnName mrk_marker.name%TYPE;
-- v_savesymbol mrk_marker.symbol%TYPE;
-- BEGIN
-- -- This procedure will process a simple marker withdrawal.
-- -- A simple marker withdrawal requires:
-- --	a) the "old" marker key
-- --	b) the reference key
-- --	c) the event reason key
-- --	c) the "new" marker symbol which does not already exist
-- --	d) the "new" marker name
-- --	e) the "add as synonym" flag
-- -- 
-- --  Since the server is not case-sensitive, the caller is
-- --  responsible for making sure the new symbol is unique and correct.
-- -- 
-- v_withdrawnName := 'withdrawn, = ' || v_newSymbol;
-- v_newKey := nextval('mrk_marker_seq') as mrk_marker_seq;
-- CREATE TEMP TABLE mrk_tmp ON COMMIT DROP
-- AS SELECT distinct m.symbol as oldSymbol, m.name as oldName, h._Refs_key as assigningRefKey
-- FROM MRK_Marker m, MRK_History h
-- WHERE m._Organism_key = 1
--      AND m._Marker_Status_key = 1
--      AND m._Marker_key = v_oldKey 
--      AND m._Marker_key = h._Marker_key
--      AND h._History_key = v_oldKey
--      AND h._Marker_Event_key = 106563604
-- ;
-- IF (SELECT count(*) FROM mrk_tmp) = 0
-- THEN
-- 	RAISE EXCEPTION E'\nMRK_simpleWithdrawal: Marker History is missing "assigned" event (%)', v_newSymbol;
-- 	RETURN;
-- END IF;
-- -- Check for duplicates; exclude cytogenetic markers
-- IF EXISTS (SELECT * FROM MRK_Marker 
-- 	WHERE _Organism_key = 1 
-- 	AND _Marker_Status_key = 1
-- 	AND _Marker_Type_key != 3
-- 	AND symbol = v_newSymbol)
-- THEN
-- 	RAISE EXCEPTION E'\nMRK_simpleWithdrawal: Duplicate Symbol (%)', v_newSymbol;
-- 	RETURN;
-- END IF;
-- v_savesymbol := symbol FROM MRK_Marker WHERE _Marker_key = v_oldKey;
-- -- Create a new marker record using the old marker record as the template
-- INSERT INTO MRK_Marker 
-- (_Marker_key, _Organism_key, _Marker_Type_key, _Marker_Status_key, symbol, name, chromosome, cmOffset, _CreatedBy_key, _ModifiedBy_key)
-- SELECT v_newKey, _Organism_key, _Marker_Type_key, 2, symbol||'_tmp', v_withdrawnName, chromosome, cmOffset, v_userKey, v_userKey
-- FROM MRK_Marker
-- WHERE _Marker_key = v_oldKey
-- ;
-- IF NOT FOUND
-- THEN
-- 	RAISE EXCEPTION E'\nMRK_simpleWithdrawal: Could not add marker (%)', v_newSymbol;
-- 	RETURN;
-- END IF;
-- -- Update the Current marker of the new marker
-- UPDATE MRK_Current 
-- SET _Current_key = v_oldKey 
-- WHERE _Current_key = v_newKey;
-- -- handled by trigger
-- --INSERT INTO MRK_Current VALUES (v_oldKey, v_newKey, now(), now());
-- -- Update old marker record with new symbol and name values
-- UPDATE MRK_Marker 
-- SET symbol = v_newSymbol, name = v_newName, _ModifiedBy_key = v_userKey, modification_date = now()
-- WHERE _Marker_key = v_oldKey;
-- -- Update old marker record with old symbol (remove '_tmp')
-- UPDATE MRK_Marker 
-- SET symbol = v_savesymbol
-- WHERE _Marker_key = v_newKey;
-- -- Update history lines
-- UPDATE MRK_History 
-- SET _History_key = v_newKey 
-- WHERE _Marker_key = v_oldKey 
-- AND _History_key = v_oldKey;
-- -- Add History line for withdrawal
-- PERFORM MRK_insertHistory (v_userKey, v_oldKey, v_newKey, v_refsKey, 106563605, v_eventReasonKey, 
-- 	(select oldName from mrk_tmp));
-- IF NOT FOUND
-- THEN
-- 	RAISE EXCEPTION E'\nMRK_simpleWithdrawal: Could not add history (withdrawal)';
-- 	RETURN;
-- END IF;
-- -- Add History line for assignment
-- PERFORM MRK_insertHistory (v_userKey, v_oldKey, v_oldKey, v_refsKey, 106563604, v_eventReasonKey, v_newName);
-- IF NOT FOUND
-- THEN
-- 	RAISE EXCEPTION E'\nMRK_simpleWithdrawal: Could not add history (assignment)';
-- 	RETURN;
-- END IF;
-- -- If marker type = 'gene' (1) and no wild type allele exists for new symbol, create it
-- --IF (SELECT _Marker_Type_key FROM MRK_Marker WHERE _Marker_key = v_newKey) = 1
-- 	--AND NOT EXISTS (SELECT 1 FROM ALL_Allele WHERE _Marker_key = v_oldKey AND isWildType = 1)
-- --THEN
-- 	--PERFORM ALL_createWildType (v_userKey, v_oldKey, v_newSymbol);
-- 	--IF NOT FOUND
-- 	--THEN
-- 		--RAISE EXCEPTION E'\nMRK_simpleWithdrawal: Could not add wild type allele';
-- 		--RETURN;
-- 	--END IF;
-- --END IF;
-- -- Convert Alleles, if necessary
-- PERFORM ALL_convertAllele (v_userKey, v_oldKey, (select oldSymbol from mrk_tmp), v_newSymbol);
-- IF NOT FOUND
-- THEN
-- 	RAISE EXCEPTION E'\nMRK_simpleWithdrawal: Could not call ALL_convertAllele';
-- 	RETURN;
-- END IF;
-- -- Insert withdrawn symbol into Synonym table
-- -- Use assigning reference
-- IF v_addAsSynonym = 1
-- THEN
-- 	PERFORM MGI_insertSynonym (v_userKey, v_oldKey, 2, 1004, 
-- 		(select oldSymbol from mrk_tmp), 
-- 		(select assigningRefKey from mrk_tmp), 1);
-- 	IF NOT FOUND
-- 	THEN
-- 		RAISE EXCEPTION E'\nMRK_simpleWithdrawal: Could not add synonym';
-- 		RETURN;
-- 	END IF;
-- END IF;
-- -- Update needsReview flag for strains
-- PERFORM PRB_setStrainReview (v_oldKey);
-- IF NOT FOUND
-- THEN
-- 	RAISE EXCEPTION E'\nMRK_simpleWithdrawal: Could not flag Strain record for needing review';
-- 	RETURN;
-- END IF;
-- RETURN;
-- END;
-- $$;


CREATE FUNCTION mgd.mrk_strainmarker_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: MRK_StrainMarker_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INPUT:
--	none
-- RETURNS:
--	NEW
IF EXISTS (SELECT * FROM MAP_Coord_Feature a
          WHERE OLD._StrainMarker_key = a._Object_key
          AND a._MGIType_key = 44)
THEN
        RAISE EXCEPTION E'Strain/Marker is referenced in MAP_Coord_Feature table';
END IF; 
DELETE FROM IMG_ImagePane_Assoc a
WHERE a._Object_key = OLD._StrainMarker_key
AND a._MGIType_key = 44
;
DELETE FROM MAP_Coord_Feature a
WHERE a._Object_key = OLD._StrainMarker_key
AND a._MGIType_key = 44
;
DELETE FROM MAP_Coordinate a
WHERE a._Object_key = OLD._StrainMarker_key
AND a._MGIType_key = 44
;
DELETE FROM MGI_Note a
WHERE a._Object_key = OLD._StrainMarker_key
AND a._MGIType_key = 44
;
DELETE FROM MGI_Property a
WHERE a._Object_key = OLD._StrainMarker_key
AND a._MGIType_key = 44
;
DELETE FROM MGI_Reference_Assoc a
WHERE a._Object_key = OLD._StrainMarker_key
AND a._MGIType_key = 44
;
DELETE FROM MGI_Synonym a
WHERE a._Object_key = OLD._StrainMarker_key
AND a._MGIType_key = 44
;
DELETE FROM ACC_Accession a
WHERE a._Object_key = OLD._StrainMarker_key
AND a._MGIType_key = 44
;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.mrk_updatekeys(v_oldkey integer, v_newkey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_userKey int;
BEGIN
-- Executed during merge withdrawal process
-- Set the preferred bit to 0 for all MGI Acc# brought over from old symbol if
-- the new symbol already contains a preferred MGI Acc#.
-- Associate all Accession numbers w/ new symbol.
IF (SELECT count(*) 
    FROM ACC_Accession 
    WHERE _MGIType_key = 2 
	AND prefixPart = 'MGI:' 
	AND _LogicalDB_key = 1 
	AND _Object_key = v_newKey 
	AND preferred = 1) > 0
THEN
	UPDATE ACC_Accession 
	SET _Object_key = v_newKey, preferred = 0
	WHERE _LogicalDB_key = 1 
	AND _MGIType_key = 2 
	AND _Object_key = v_oldKey
	;
END IF;
-- remove any Accession records belonging to old marker
-- which already exist for new marker
-- that is, remove duplicates before updating keys
DELETE 
FROM ACC_Accession
USING ACC_Accession new
WHERE ACC_Accession._MGIType_key = 2
AND ACC_Accession._Object_key = v_oldKey
AND ACC_Accession.accID = new.accID
AND ACC_Accession._LogicalDB_key = new._LogicalDB_key
AND new._Object_key = v_newKey
AND new._MGIType_key = 2
;
UPDATE ACC_Accession 
SET _Object_key = v_newKey
WHERE _MGIType_key = 2 AND _Object_key = v_oldKey
;
-- Associate classes, other names, references w/ new symbol 
UPDATE ALL_Allele SET _Marker_key = v_newKey WHERE _Marker_key = v_oldKey ;
UPDATE ALL_Knockout_Cache SET _Marker_key = v_newKey WHERE _Marker_key = v_oldKey ;
UPDATE MGI_Synonym SET _Object_key = v_newKey WHERE _MGIType_key = 2 AND _Object_key = v_oldKey ;
UPDATE MRK_Notes SET _Marker_key = v_newKey WHERE _Marker_key = v_oldKey ;
-- Update all auxiliary references to old symbol w/ new symbol 
UPDATE CRS_Matrix SET _Marker_key = v_newKey WHERE _Marker_key = v_oldKey ;
UPDATE CRS_References SET _Marker_key = v_newKey WHERE _Marker_key = v_oldKey ;
UPDATE GXD_AllelePair SET _Marker_key = v_newKey WHERE _Marker_key = v_oldKey ;
UPDATE GXD_AlleleGenotype SET _Marker_key = v_newKey WHERE _Marker_key = v_oldKey ;
UPDATE GXD_Assay SET _Marker_key = v_newKey WHERE _Marker_key = v_oldKey ;
UPDATE GXD_Expression SET _Marker_key = v_newKey WHERE _Marker_key = v_oldKey ;
UPDATE WKS_Rosetta SET _Marker_key = v_newKey WHERE _Marker_key = v_oldKey ;
-- GXD_AntibodyMarker may contain potential duplicates 
DELETE 
FROM GXD_AntibodyMarker
USING GXD_AntibodyMarker p2
WHERE GXD_AntibodyMarker._Marker_key = v_oldKey 
AND GXD_AntibodyMarker._Antibody_key = p2._Antibody_key 
AND p2._Marker_key = v_newKey
;
UPDATE GXD_AntibodyMarker SET _Marker_key = v_newKey WHERE _Marker_key = v_oldKey ;
-- GXD_Index may contain potential duplicates 
IF EXISTS (select 1 FROM GXD_Index WHERE _Marker_key = v_newKey)
THEN
    -- merge duplicate index stage data 
    UPDATE GXD_Index_Stages
    SET _Index_key = n._Index_key
    FROM GXD_Index n, GXD_Index o
    WHERE n._Marker_key = v_newKey
    AND n._Refs_key = o._Refs_key
    AND o._Marker_key = v_oldKey
    AND o._Index_key = GXD_Index_Stages._Index_key
    AND not exists (select 1 FROM GXD_Index_Stages ns
    WHERE n._Index_key = ns._Index_key
    AND ns._IndexAssay_key = GXD_Index_Stages._IndexAssay_key
    AND ns._StageID_key = GXD_Index_Stages._StageID_key)
    ;
    
    DELETE 
    FROM GXD_Index
    WHERE exists (select 1 FROM GXD_Index n
    WHERE n._Marker_key = v_newKey
    AND n._Refs_key = GXD_Index._Refs_key
    AND GXD_Index._Marker_key = v_oldKey)
    ;
END IF;
-- this handles non-duplicate updates 
UPDATE GXD_Index SET _Marker_key = v_newKey WHERE _Marker_key = v_oldKey ;
UPDATE MLD_Expt_Marker SET _Marker_key = v_newKey WHERE _Marker_key = v_oldKey ;
UPDATE MLD_Concordance SET _Marker_key = v_newKey WHERE _Marker_key = v_oldKey ;
UPDATE MLD_MC2point SET _Marker_key_1 = v_newKey WHERE _Marker_key_1 = v_oldKey ;
UPDATE MLD_MC2point SET _Marker_key_2 = v_newKey WHERE _Marker_key_2 = v_oldKey ;
UPDATE MLD_RI2Point SET _Marker_key_1 = v_newKey WHERE _Marker_key_1 = v_oldKey ;
UPDATE MLD_RI2Point SET _Marker_key_2 = v_newKey WHERE _Marker_key_2 = v_oldKey ;
UPDATE MLD_RIData SET _Marker_key = v_newKey WHERE _Marker_key = v_oldKey ;
UPDATE MLD_Statistics SET _Marker_key_1 = v_newKey WHERE _Marker_key_1 = v_oldKey ;
UPDATE MLD_Statistics SET _Marker_key_2 = v_newKey WHERE _Marker_key_2 = v_oldKey ;
PERFORM PRB_setStrainReview (v_oldKey);
 
UPDATE PRB_Strain_Marker SET _Marker_key = v_newKey WHERE _Marker_key = v_oldKey ;
-- PRB_Marker may contain potential duplicates 
DELETE 
FROM PRB_Marker
USING PRB_Marker p2
WHERE PRB_Marker._Marker_key = v_oldKey 
AND PRB_Marker._Probe_key = p2._Probe_key 
AND p2._Marker_key = v_newKey
;
UPDATE PRB_Marker SET _Marker_key = v_newKey WHERE _Marker_key = v_oldKey ;
UPDATE PRB_RFLV SET _Marker_key = v_newKey WHERE _Marker_key = v_oldKey ;
-- Update Annotations - 01/23/2002 - TR 2867 - MGI 2.8 - GO/Marker 
PERFORM VOC_mergeAnnotations (1000, v_oldKey, v_newKey);
-- Update Annotations - 06/17/2003 - TR 4902 - InterPro/Marker 
PERFORM VOC_mergeAnnotations (1003, v_oldKey, v_newKey);
-- Delete roll-up annotations
-- 1015 : Mammalian Phenotype/Marker (Derived)
-- 1023 : DO/Marker (Derived)
DELETE FROM VOC_Annot WHERE _AnnotType_key = 1015 AND _Object_key = v_oldKey;
DELETE FROM VOC_Annot WHERE _AnnotType_key = 1023 AND _Object_key = v_oldKey;
-- Delete MRK_ClusterMember from old Symbol
DELETE FROM MRK_ClusterMember where _Marker_key = v_oldKey;
-- Move non-duplicate Marker References to the new symbol 
UPDATE MGI_Reference_Assoc
SET _Object_key = v_newKey
WHERE MGI_Reference_Assoc._MGIType_key = 2
AND MGI_Reference_Assoc._Object_key = v_oldKey
AND not exists (select 1 FROM MGI_Reference_Assoc r2
WHERE r2._MGIType_key = 2
AND r2._Object_key = v_newKey
AND r2._Refs_key = MGI_Reference_Assoc._Refs_key)
;
-- Delete remaining References from old Symbol 
DELETE FROM MGI_Reference_Assoc WHERE _MGIType_key = 2 AND _Object_key = v_oldKey ;
-- Reload cache tables for both old and new symbols 
PERFORM MRK_reloadLocation (v_oldKey);
PERFORM MRK_reloadLocation (v_newKey);
END;
$$;


-- CREATE FUNCTION mgd.prb_ageminmax(v_age text, OUT v_agemin numeric, OUT v_agemax numeric) RETURNS record
--     LANGUAGE plpgsql
--     AS $$
-- DECLARE
-- saveAge prb_source.age%TYPE;
-- stem prb_source.age%TYPE;
-- timeUnit varchar(25);
-- timeRange varchar(25);
-- i integer;
-- idx integer;
-- idx2 integer;
-- minValue numeric;
-- maxValue numeric;
-- BEGIN
-- saveAge := v_age;
-- IF v_age = 'Not Specified' 
-- 	or v_age = 'Not Applicable' 
-- 	or v_age = 'Not Loaded'
-- 	or v_age = 'Not Resolved'
-- THEN
-- 	v_ageMin := -1.0;
-- 	v_ageMax := -1.0;
-- ELSIF v_age = 'embryonic'
-- THEN
-- 	v_ageMin := 0.0;
-- 	v_ageMax := 21.0;
-- ELSIF v_age = 'postnatal'
-- THEN
-- 	v_ageMin := 21.01;
-- 	v_ageMax := 1846.0;
-- ELSIF v_age = 'postnatal newborn'
-- THEN
-- 	v_ageMin := 21.01;
-- 	v_ageMax := 25.0;
-- ELSIF v_age = 'postnatal adult'
-- THEN
-- 	v_ageMin := 42.01;
-- 	v_ageMax := 1846.0;
-- ELSIF v_age = 'postnatal day'
-- 	or v_age = 'postnatal week'
-- 	or v_age = 'postnatal month'
-- 	or v_age = 'postnatal year'
-- 	or v_age = 'embryonic day'
-- THEN
--         RAISE NOTICE E'Invalid Age Value: "%"\n', saveAge;
-- 	RETURN;
-- -- parse the age into 3 parts:          
-- --     stem (embryonic, postnatal),     
-- --     time unit (day, week, month year) 
-- --     time range (x   x,y,z   x-y)     
-- ELSE
--     minValue := 5000.0;
--     maxValue := -1.0;
--     i := 0;
--     WHILE v_age IS NOT NULL AND i < 3 LOOP
--         idx := position(' ' in v_age);
--         IF idx > 0 THEN
--             IF i = 0 THEN
--                 stem := substring(v_age, 1, idx - 1);
--             ELSIF i = 1 THEN
--                 timeUnit := substring(v_age, 1, idx - 1);
--             ELSIF i = 1 THEN
--                 timeRange := substring(v_age, 1, char_length(v_age));
-- 	    END IF;
--             v_age := substring(v_age, idx + 1, char_length(v_age));
--         ELSE
--             timeRange := substring(v_age, 1, char_length(v_age));
--             v_age := null;
--         END IF;
--         i := i + 1;
--     END LOOP; -- WHILE v_age != null...
--     -- once the stem, time unit and time range have been parsed,
--     -- determine the format of the time range and process accordingly.
--     -- format is:                
--     --     embryonic day x,y,z    
--     --     embryonic day x-y,z     
--     --     embryonic day x,y-z       
--     --                               
--     -- we assume that x is the min and z is the max 
--     --                                              
--     -- NOTE:  there are very fiew of these cases (29 as of 07/23/2003) 
--     IF position(',' in timeRange) > 0 THEN
--         WHILE timeRange IS NOT NULL LOOP
--             idx := position(',' in timeRange);
--             idx2 := position('-' in timeRange);
--             IF idx2 > 0 THEN
--                 -- the x-y of an "x-y,z..." 
--                 IF idx > idx2 THEN
--                     IF minValue > substring(timeRange, 1, idx2 - 1)::NUMERIC THEN
--                         minValue := substring(timeRange, 1, idx2 - 1)::NUMERIC;
-- 		    END IF;
--                     IF maxValue < substring(timeRange, idx2 + 1, idx - idx2 - 1)::NUMERIC THEN
--                         maxValue := substring(timeRange, idx2 + 1, idx - idx2 - 1)::NUMERIC;
-- 		    END IF;
--                 -- more timeRanges (more commas)
--                 ELSIF idx > 0 THEN
--                     IF minValue > substring(timeRange, 1, idx - 1)::NUMERIC THEN
--                         minValue := substring(timeRange, 1, idx - 1)::NUMERIC;
-- 		    END IF;
--                     IF maxValue < convert(numeric, substring(timeRange, 1, idx - 1)) THEN
--                         maxValue := substring(timeRange, 1, idx - 1)::NUMERIC;
-- 		    END IF;
--                 -- last timeRange
--                 ELSE
--                     IF minValue > substring(timeRange, 1, idx2 - 1)::NUMERIC THEN
--                         minValue := substring(timeRange, 1, idx2 - 1)::NUMERIC;
-- 		    END IF;
--                     IF maxValue < substring(timeRange, idx2 + 1, char_length(timeRange))::NUMERIC THEN
--                         maxValue := substring(timeRange, idx2 + 1, char_length(timeRange))::NUMERIC;
-- 		    END IF;
--                 END IF;
--             ELSE
--                 -- more timeRanges
--                 IF idx > 0 THEN
--                     IF minValue > substring(timeRange, 1, idx - 1)::NUMERIC THEN
--                         minValue := substring(timeRange, 1, idx - 1)::NUMERIC;
--                     END IF;
--                     IF maxValue < substring(timeRange, 1, idx - 1)::NUMERIC THEN
--                         maxValue := substring(timeRange, 1, idx - 1)::NUMERIC;
--                     END IF;
--                 -- last timeRange
--                 ELSE
--                     IF minValue > timeRange::NUMERIC THEN
--                         minValue := timeRange::NUMERIC;
--                     END IF;
--                     IF maxValue < timeRange::NUMERIC THEN
--                         maxValue := timeRange::NUMERIC;
-- 		    END IF;
--                 END IF;
--             END IF;
--             IF position(',' in timeRange) = 0 THEN
--                 timeRange := null;
--             ELSE
--                 timeRange := substring(timeRange, idx + 1, char_length(timeRange));
--             END IF;
--         END LOOP;
--     ELSE
--         -- format is:                     
--         --     embryonic day x-y          
--         idx := position('-' in timeRange);
--         IF idx > 0 THEN
--             minValue := substring(timeRange, 1, idx - 1)::NUMERIC;
--             maxValue := substring(timeRange, idx + 1, char_length(timeRange))::NUMERIC;
--         -- format is:                    
--         --     embryonic day x          
--         ELSE
--             minValue := timeRange::NUMERIC;
--             maxValue := timeRange::NUMERIC;
--         END IF;
--     END IF; -- end IF position(',' in timeRange) > 0 THEN
--     IF minValue is null or maxValue is null THEN
--         RETURN;
--     END IF;
--     -- multiply postnatal values according to time unit
--     IF stem = 'postnatal' THEN
--         IF timeUnit = 'day' THEN
--             v_ageMin := minValue + 21.01;
--             v_ageMax := maxValue + 21.01;
--         ELSEIF timeUnit = 'week' THEN
--             v_ageMin := minValue * 7 + 21.01;
--             v_ageMax := maxValue * 7 + 21.01;
--         ELSEIF timeUnit = 'month' THEN
--             v_ageMin := minValue * 30 + 21.01;
--             v_ageMax := maxValue * 30 + 21.01;
--         ELSEIF timeUnit = 'year' THEN
--             v_ageMin := minValue * 365 + 21.01;
--             v_ageMax := maxValue * 365 + 21.01;
--         END IF;
--     ELSE
--         v_ageMin := minValue;
--         v_ageMax := maxValue;
--     END IF;
-- END IF; -- final
-- IF stem = 'Not' AND v_ageMin >= 0
-- THEN
--     RAISE NOTICE E'Invalid Age Value: "%"\n', saveAge;
--     RETURN;
-- END IF;
-- IF stem = 'embryonic' AND timeUnit IS NULL AND timeRange IS NOT NULL
-- THEN
--     RAISE NOTICE E'Invalid Age Value: "%"\n', saveAge;
--     RETURN;
-- END IF;
-- IF v_ageMin IS NULL AND v_ageMax IS NULL THEN
--     RAISE NOTICE E'Invalid Age Value: "%"\n', saveAge;
--     RETURN;
-- END IF;
-- RETURN;
-- END;
-- $$;


CREATE FUNCTION mgd.prb_getstrainbyreference(v_refskey integer) RETURNS TABLE(_strain_key integer, strain text)
    LANGUAGE plpgsql
    AS $$
BEGIN
CREATE TEMP TABLE strains ON COMMIT DROP
AS SELECT DISTINCT m._Strain_key
FROM MLD_Expts e, MLD_InSitu m
WHERE e._Refs_key = v_refsKey
AND e._Expt_key = m._Expt_key
UNION
SELECT DISTINCT m._Strain_key
FROM MLD_Expts e, MLD_FISH m
WHERE e._Refs_key = v_refsKey
AND e._Expt_key = m._Expt_key
UNION
SELECT DISTINCT c._femaleStrain_key
FROM MLD_Expts e, MLD_Matrix m, CRS_Cross c
WHERE e._Refs_key = v_refsKey
AND e._Expt_key = m._Expt_key
AND m._Cross_key = c._Cross_key
UNION
SELECT DISTINCT c._maleStrain_key
FROM MLD_Expts e, MLD_Matrix m, CRS_Cross c
WHERE e._Refs_key = v_refsKey
AND e._Expt_key = m._Expt_key
AND m._Cross_key = c._Cross_key
UNION
SELECT DISTINCT c._StrainHO_key
FROM MLD_Expts e, MLD_Matrix m, CRS_Cross c
WHERE e._Refs_key = v_refsKey
AND e._Expt_key = m._Expt_key
AND m._Cross_key = c._Cross_key
UNION
SELECT DISTINCT c._StrainHT_key
FROM MLD_Expts e, MLD_Matrix m, CRS_Cross c
WHERE e._Refs_key = v_refsKey
AND e._Expt_key = m._Expt_key
AND m._Cross_key = c._Cross_key
UNION
SELECT DISTINCT s._Strain_key
FROM GXD_Genotype s, GXD_Expression x
WHERE x._Refs_key = v_refsKey
AND x._Genotype_key = s._Genotype_key
UNION
SELECT DISTINCT s._Strain_key
FROM PRB_Reference r, PRB_RFLV v, PRB_Allele a, PRB_Allele_Strain s
WHERE r._Refs_key = v_refsKey
AND r._Reference_key = v._Reference_key
AND v._RFLV_key = a._RFLV_key
AND a._Allele_key = s._Allele_key
UNION
SELECT DISTINCT a._Strain_key
FROM ALL_Allele a, MGI_Reference_Assoc r
WHERE r._Refs_key = v_refsKey
AND r._Object_key = a._Allele_key
AND r._MGIType_key = 11
;
 
RETURN QUERY
SELECT t._Strain_key, t.strain
FROM strains s, PRB_Strain t
WHERE s._Strain_key = t._Strain_key
ORDER BY t.strain
;
END;
$$;


CREATE FUNCTION mgd.prb_getstraindatasets(v_strainkey integer) RETURNS TABLE(accid text, dataset text)
    LANGUAGE plpgsql
    AS $$
BEGIN
-- Select all Probes AND Data Sets for given Strain
 
CREATE TEMP TABLE source_tmp ON COMMIT DROP
AS SELECT _Source_key, _Strain_key
FROM PRB_Source
WHERE _Strain_key = v_strainKey
ORDER BY _Source_key
;
CREATE UNIQUE index idx_Source_key ON source_tmp(_Source_key);
CREATE INDEX index_Strain_key ON source_tmp(_Strain_key);
IF (SELECT COUNT(*) FROM source_tmp s, PRB_Probe p WHERE s._Source_key = p._Source_key) > 10000
THEN
	RAISE EXCEPTION 'More than 10000 objects_tmp are associated with this Strain.';
	RETURN;
END IF;
CREATE TEMP TABLE objects_tmp ON COMMIT DROP
AS SELECT p._Probe_key as _Object_key, 'Molecular Segment' as dataSet
FROM source_tmp s, PRB_Probe p
WHERE s._Source_key = p._Source_key
UNION
SELECT DISTINCT a._Antigen_key, 'Antigen' as dataSet
FROM source_tmp s, GXD_Antigen a 
WHERE s._Source_key = a._Source_key
UNION
SELECT DISTINCT a._Allele_key, 'Allele' as dataSet
FROM ALL_Allele a
WHERE a._Strain_key = v_strainKey
UNION
SELECT DISTINCT a._Genotype_key, 'Genotype' as dataSet
FROM GXD_Genotype a
WHERE a._Strain_key = v_strainKey
UNION
SELECT DISTINCT a._RISet_key, 'RI Set' as dataSet
FROM RI_RISet a 
WHERE a._Strain_key_1 = v_strainKey
UNION
SELECT DISTINCT a._RISet_key, 'RI Set' as dataSet
FROM RI_RISet a 
WHERE a._Strain_key_2 = v_strainKey
UNION
SELECT DISTINCT a._Sequence_key, 'Sequence' as dataSet
FROM PRB_Source s, SEQ_Source_Assoc a
WHERE s._Strain_key = v_strainKey
AND s._Source_key = a._Source_key
;
RETURN QUERY
SELECT a.accID, p.dataSet
FROM objects_tmp p, PRB_Acc_View a
WHERE p.dataSet = 'Molecular Segment'
AND p._Object_key = a._Object_key
AND a.prefixPart = 'MGI:'
AND a._LogicalDB_key = 1
AND a.preferred = 1
UNION
SELECT a.accID, p.dataSet
FROM objects_tmp p, GXD_Antigen_Acc_View a
WHERE p.dataSet = 'Antigen'
AND p._Object_key = a._Object_key
AND a.prefixPart = 'MGI:'
AND a._LogicalDB_key = 1
AND a.preferred = 1
UNION
SELECT a.accID, p.dataset
FROM objects_tmp p, ALL_Acc_View a
WHERE p.dataSet = 'Allele'
AND p._Object_key = a._Object_key
AND a.prefixPart = 'MGI:'
AND a._LogicalDB_key = 1
AND a.preferred = 1
UNION
SELECT a.accID, p.dataset
FROM objects_tmp p, GXD_Genotype_Acc_View a
WHERE p.dataSet = 'Genotype'
AND p._Object_key = a._Object_key
AND a.prefixPart = 'MGI:'
AND a._LogicalDB_key = 1
AND a.preferred = 1
UNION
SELECT p._Object_key::char(30), p.dataset
FROM objects_tmp p
WHERE p.dataSet = 'RI Set'
UNION
SELECT a.accID, p.dataset
FROM objects_tmp p, ACC_Accession a
WHERE p.dataSet = 'Sequence'
AND p._Object_key = a._Object_key
AND a._MGIType_key = 19 
;
END;
$$;


CREATE FUNCTION mgd.prb_getstrainreferences(v_strainkey integer) RETURNS TABLE(jnum integer, dataset text, jnumid text)
    LANGUAGE plpgsql
    AS $$
BEGIN
RETURN QUERY
SELECT a.jnum, r.dataSet, a.jnumid
FROM prb_strain_reference_view r, BIB_View a
WHERE r._Strain_key = v_strainKey
AND r._Refs_key = a._Refs_key
ORDER BY a.jnum
;
END;
$$;


CREATE FUNCTION mgd.prb_insertreference(v_userkey integer, v_refskey integer, v_probekey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
-- Insert record into PRB_Reference if _Refs_key/_Probe_key pair does not already exist
IF (SELECT count(*) FROM PRB_Reference WHERE _Refs_key = v_refsKey AND _Probe_key = v_probeKey) > 0
THEN
	RETURN;
END IF;
INSERT INTO PRB_Reference
(_Reference_key, _Probe_key, _Refs_key, hasRmap, hasSequence, _CreatedBy_key, _ModifiedBy_key)
VALUES (nextval('prb_reference_seq'), v_probeKey, v_refsKey, 0, 0, v_userKey, v_userKey)
;
END;
$$;


CREATE FUNCTION mgd.prb_marker_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: PRB_Marker_insert()
-- DESCRIPTOIN:
-- INPUT:
--	none
-- RETURNS:
--	NEW
-- Cannot annotate using the Auto-E reference/J:85324 (86302)
IF (SELECT current_user) NOT IN ('mgd_dbo', 'dbo')
   AND
   (NEW._Refs_key = 86302)
THEN
        RAISE EXCEPTION E'PRB_Marker_insert: Cannot use J:85324 (auto-E) reference.';
END IF;
-- Relationship cannot be null
IF (NEW.relationship IS NULL)
THEN
        RAISE EXCEPTION E'PRB_Marker_insert: Relationship cannot be NULL.';
END IF;
-- Relationship must be 'H' for Probes of non-mouse source
-- not a primer
IF EXISTS (SELECT * FROM PRB_Probe p, PRB_Source s
        WHERE NEW._Probe_key = p._Probe_key
        and p._SegmentType_key != 63473
        and p._Source_key = s._Source_key
        and s._Organism_key != 1
        and (NEW.relationship != 'H' or NEW.relationship IS NULL))
THEN
        RAISE EXCEPTION E'PRB_Marker_insert: Relationship Must be \'H\'';
END IF;
-- Relationship 'P' can only be used during an EST bulk load
IF (NEW.relationship = 'P')
THEN
        RAISE EXCEPTION E'PRB_Marker_insert: Relationship \'P\' can only be used during EST load';
END IF;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.prb_marker_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: PRB_Marker_update()
-- DESCRIPTOIN:
--	a) Relationship rules : see below for details
--	b) propagate Marker changes to PRB_RFLV
-- INPUT:
--	none
-- RETURNS:
--	NEW
-- Cannot annotate using the Auto-E reference/J:85324 (86302)
IF (SELECT current_user) NOT IN ('mgd_dbo', 'dbo')
   AND
   (NEW._Refs_key = 86302)
THEN
        RAISE EXCEPTION E'PRB_Marker_update: Cannot use J:85324 (auto-E) reference.';
END IF;
-- Relationship cannot be changed from non-null to null
IF (OLD.relationship IS NOT NULL AND NEW.relationship IS NULL)
THEN
        RAISE EXCEPTION E'PRB_Marker_update: Relationship cannot be NULL.';
END IF;
-- Relationship must be 'H' for Probes of non-mouse source
-- not a primer
IF EXISTS (SELECT * FROM PRB_Probe p, PRB_Source s
        WHERE NEW._Probe_key = p._Probe_key
        and p._SegmentType_key != 63473
        and p._Source_key = s._Source_key
        and s._Organism_key != 1
        and (NEW.relationship != 'H' or NEW.relationship IS NULL))
THEN
        RAISE EXCEPTION E'PRB_Marker_update: Relationship must be \'H\'';
END IF;
-- Allow update of 'P' to other relationship
-- Disallow update of other relationship to 'P'
-- Only check on individual inserts
if (NEW.relationship = 'P' AND NEW._Marker_key = OLD._Marker_key)
   OR
   (NEW._Marker_key != OLD._Marker_key AND NEW.relationship = 'P' AND OLD.relationship != 'P')
THEN
        RAISE EXCEPTION E'PRB_Marker_update: Relationship \'P\' can only be used during EST load';
END IF;
-- Propagate changes to _Marker_key to RFLV tables
UPDATE PRB_RFLV
SET _Marker_key = NEW._Marker_key
FROM PRB_Reference
WHERE NEW._Probe_key = PRB_Reference._Probe_key
AND PRB_Reference._Reference_key = PRB_RFLV._Reference_key
AND PRB_RFLV._Marker_key = OLD._Marker_key
;
RETURN NEW;
END;
$$;


-- CREATE FUNCTION mgd.prb_mergestrain(v_oldstrainkey integer, v_newstrainkey integer) RETURNS void
--     LANGUAGE plpgsql
--     AS $$
-- DECLARE
-- v_alleleKey int;
-- v_probe text;
-- v_jnum text;
-- v_strainAttributesType int;
-- v_oldNeedsReviewSum int;
-- v_newNeedsReviewSum int;
-- v_jaxRegistryNew acc_accession.accID%TYPE;
-- v_jaxRegistryOld acc_accession.accID%TYPE;
-- v_strainmergeKey int;
-- v_noteTypeKey int;
-- v_note1 text;
-- v_note2 text;
-- v_nextKey int;
-- v_synTypeKey int;
-- v_nextSeqKey int;
-- BEGIN
-- -- Update old Strain key to new Strain key
-- -- in all relevant tables which contain a Strain key.
-- -- When finished, remove the Strain record for the old Strain key.
-- IF v_oldStrainKey = v_newStrainKey
-- THEN
-- 	RAISE EXCEPTION E'PRB_mergeStrain: Cannot merge a Strain into itself.';
-- 	RETURN;
-- END IF;
-- -- Check for valid merge conditions
-- -- disallowed:
-- --	private -> public
-- --      public -> private
-- --      Standard -> Non Standard 
-- IF (SELECT private FROM PRB_Strain WHERE _Strain_key = v_oldStrainKey) = 1
--    AND
--    (SELECT private FROM PRB_Strain WHERE _Strain_key = v_newStrainKey) = 0
-- THEN
-- 	RAISE EXCEPTION E'PRB_mergeStrain: Cannot merge Private Strain into Public Strain.';
-- 	RETURN;
-- END IF;
-- IF (SELECT private FROM PRB_Strain WHERE _Strain_key = v_oldStrainKey) = 0
--    AND
--    (SELECT private FROM PRB_Strain WHERE _Strain_key = v_newStrainKey) = 1
-- THEN
-- 	RAISE EXCEPTION E'PRB_mergeStrain: Cannot merge Public Strain into Private Strain';
-- 	RETURN;
-- END IF;
-- IF (SELECT standard FROM PRB_Strain WHERE _Strain_key = v_oldStrainKey) = 1
--    AND
--    (SELECT standard FROM PRB_Strain WHERE _Strain_key = v_newStrainKey) = 0
-- THEN
-- 	RAISE EXCEPTION E'PRB_mergeStrain: Cannot merge Standard Strain into Non-Standard Strain';
-- 	RETURN;
-- END IF;
-- -- Check for potential duplicate Probe RFLV Entries
-- FOR v_alleleKey IN
-- SELECT DISTINCT _Allele_key
-- FROM PRB_Allele_Strain
-- WHERE _Strain_key in (v_oldStrainKey, v_newStrainkey)
-- GROUP by _Allele_key having count(*) > 1
-- LOOP
-- 	SELECT p.name as v_probe, b.accID as v_jnum
-- 	FROM PRB_Allele a, PRB_RFLV v, PRB_Reference  r, PRB_Probe p, BIB_Acc_View b
-- 	WHERE a._Allele_key = v_alleleKey and
-- 	      a._RFLV_key = v._RFLV_key and
-- 	      v._Reference_key = r._Reference_key and
-- 	      r._Probe_key = p._Probe_key and
-- 	      r._Refs_key = b._Object_key and
-- 	      b.prefixPart = 'J:' and
-- 	      b._LogicalDB_key = 1
--         ;
-- 	RAISE EXCEPTION E'PRB_mergeStrain: This merge would create a duplicate entry for Probe %, %.', v_probe, v_jnum;
-- 	RETURN;
-- END LOOP;
-- -- all Strains must have same symbols
-- IF EXISTS (SELECT m1.* FROM PRB_Strain_Marker m1
--            WHERE m1._Strain_key = v_newStrainKey
-- 	   AND NOT EXISTS
-- 	   (SELECT m2.* FROM PRB_Strain_Marker m2
-- 	    WHERE m2._Strain_key = v_oldStrainKey AND
-- 	    m2._Marker_key = m1._Marker_key))
-- THEN
-- 	RAISE EXCEPTION E'PRB_mergeStrain: Incorrect and Correct Strains must have the same Marker Symbols.';
-- 	RETURN;
-- END IF;
-- IF EXISTS (SELECT m1.* FROM PRB_Strain_Marker m1
--            WHERE m1._Strain_key = v_oldStrainKey
-- 	   AND NOT EXISTS
-- 	   (SELECT m2.* FROM PRB_Strain_Marker m2
-- 	    WHERE m2._Strain_key = v_newStrainKey AND
-- 	    m2._Marker_key = m1._Marker_key))
-- THEN
-- 	RAISE EXCEPTION E'PRB_mergeStrain: Incorrect and Correct Strains must have the same Marker Symbols.';
-- 	RETURN;
-- END IF;
-- -- both Strains must have the same Strain Attributes
-- --v_strainAttributesType := _AnnotType_key from VOC_AnnotType WHERE name = 'Strain/Attributes';
-- --v_strainmergeKey := _User_key from MGI_User WHERE login = 'strainmerge';
-- v_strainAttributesType := 1009;
-- v_strainmergeKey := 1400;
-- IF EXISTS (SELECT m1.* FROM VOC_Annot m1
--            WHERE m1._Object_key = v_newStrainKey
-- 	   AND m1._AnnotType_key = v_strainAttributesType
-- 	   AND NOT EXISTS
-- 	   (SELECT m2.* FROM VOC_Annot m2
-- 	    WHERE m2._Object_key = v_oldStrainKey
-- 	    AND m2._AnnotType_key = v_strainAttributesType
-- 	    AND m2._Term_key = m1._Term_key))
-- THEN
-- 	RAISE EXCEPTION E'PRB_mergeStrain: Incorrect and Correct Strains must have the same Strain Attributes.';
-- 	RETURN;
-- END IF;
-- IF EXISTS (SELECT m1.* FROM VOC_Annot m1
--            WHERE m1._Object_key = v_oldStrainKey
-- 	   AND m1._AnnotType_key = v_strainAttributesType
-- 	   AND NOT EXISTS
-- 	   (SELECT m2.* FROM VOC_Annot m2
-- 	    WHERE m2._Object_key = v_newStrainKey
-- 	    AND m2._AnnotType_key = v_strainAttributesType
-- 	    AND m2._Term_key = m1._Term_key))
-- THEN
-- 	RAISE EXCEPTION E'PRB_mergeStrain: Incorrect and Correct Strains must have the same Strain Attributes.';
-- 	RETURN;
-- END IF;
-- -- both Strains must have the same Species value
-- IF (SELECT _Species_key FROM PRB_Strain WHERE _Strain_key = v_newStrainKey) !=
--    (SELECT _Species_key FROM PRB_Strain WHERE _Strain_key = v_oldStrainKey)
-- THEN
-- 	RAISE EXCEPTION E'PRB_mergeStrain: Incorrect and Correct Strains must have the same Species.';
-- 	RETURN;
-- END IF;
-- -- both Strains must have the same Needs Review values
-- v_oldNeedsReviewSum := sum(_Term_key) from PRB_Strain_NeedsReview_View WHERE _Object_key = v_oldStrainKey;
-- v_newNeedsReviewSum := sum(_Term_key) from PRB_Strain_NeedsReview_View WHERE _Object_key = v_newStrainKey;
-- IF (v_oldNeedsReviewSum != v_newNeedsReviewSum)
-- THEN
-- 	RAISE EXCEPTION E'PRB_mergeStrain: Incorrect and Correct Strains must have the same Needs Review values.';
-- 	RETURN;
-- END IF;
-- -- JAX Registry - must be equal OR use the one that exists
-- v_jaxRegistryNew := NULL;
-- v_jaxRegistryOld := NULL;
-- IF EXISTS (SELECT accID FROM ACC_Accession 
-- 	   WHERE _Object_key = v_newStrainKey and _LogicalDB_Key = 22 and _MGIType_key = 10)
-- THEN
-- 	v_jaxRegistryNew := accID FROM ACC_Accession 
-- 		WHERE _Object_key = v_newStrainKey and _LogicalDB_Key = 22 and _MGIType_key = 10;
-- END IF;
-- IF EXISTS (SELECT _Accession_key FROM ACC_Accession
--            WHERE _Object_key = v_oldStrainKey and _LogicalDB_Key = 22 and _MGIType_key = 10)
-- THEN
-- 	v_jaxRegistryOld := accID FROM ACC_Accession
-- 		WHERE _Object_key = v_oldStrainKey and _LogicalDB_Key = 22 and _MGIType_key = 10;
-- END IF;
-- IF (v_jaxRegistryNew != NULL AND v_jaxRegistryOld != NULL AND v_jaxRegistryNew != v_jaxRegistryOld)
-- THEN
-- 	RAISE EXCEPTION E'PRB_mergeStrain: Incorrect and Correct Strains must have the same JAX Registry Number.';
-- 	RETURN;
-- ELSIF (v_jaxRegistryOld != NULL)
-- THEN
--     v_jaxRegistryNew := v_jaxRegistryOld;
-- END IF;
-- -- Set the preferred bit to 0 for all MGI Acc# brought over from old strain.
-- UPDATE ACC_Accession 
-- SET _Object_key = v_newStrainKey, preferred = 0
-- WHERE _LogicalDB_key = 1 and _MGIType_key = 10 and _Object_key = v_oldStrainKey
-- ;
-- -- remove any Accession records belonging to old strain 
-- -- which already exist for new strain 
-- -- that is, remove duplicates before updating keys 
-- DELETE FROM ACC_Accession old
-- USING ACC_Accession new
-- WHERE old._MGIType_key = 10
-- AND old._Object_key = v_oldStrainKey
-- AND old.accID = new.accID
-- AND old._LogicalDB_key = new._LogicalDB_key
-- AND new._Object_key = v_newStrainKey
-- AND new._MGIType_key = 10
-- ;
-- UPDATE ACC_Accession 
-- SET _Object_key = v_newStrainKey
-- WHERE _MGIType_key = 10 and _Object_key = v_oldStrainKey
-- ;
-- UPDATE ALL_Allele
-- SET _Strain_key = v_newStrainKey, _ModifiedBy_key = v_strainmergeKey, modification_date = now()
-- WHERE _Strain_key = v_oldStrainKey
-- ;
-- UPDATE ALL_CellLine
-- SET _Strain_key = v_newStrainKey
-- WHERE _Strain_key = v_oldStrainKey
-- ;
-- UPDATE PRB_Source
-- SET _Strain_key = v_newStrainKey, _ModifiedBy_key = v_strainmergeKey, modification_date = now()
-- WHERE _Strain_key = v_oldStrainKey
-- ;
-- UPDATE PRB_Allele_Strain
-- SET _Strain_key = v_newStrainKey, _ModifiedBy_key = v_strainmergeKey, modification_date = now()
-- WHERE _Strain_key = v_oldStrainKey
-- ;
-- UPDATE MLD_FISH
-- SET _Strain_key = v_newStrainKey
-- WHERE _Strain_key = v_oldStrainKey
-- ;
-- UPDATE MLD_InSitu
-- SET _Strain_key = v_newStrainKey
-- WHERE _Strain_key = v_oldStrainKey
-- ;
-- UPDATE CRS_Cross
-- SET _femaleStrain_key = v_newStrainKey
-- WHERE _femaleStrain_key = v_oldStrainKey
-- ;
-- UPDATE CRS_Cross
-- SET _maleStrain_key = v_newStrainKey
-- WHERE _maleStrain_key = v_oldStrainKey
-- ;
-- UPDATE CRS_Cross
-- SET _StrainHO_key = v_newStrainKey
-- WHERE _StrainHO_key = v_oldStrainKey
-- ;
-- UPDATE CRS_Cross
-- SET _StrainHT_key = v_newStrainKey
-- WHERE _StrainHT_key = v_oldStrainKey
-- ;
-- UPDATE GXD_Genotype
-- SET _Strain_key = v_newStrainKey, _ModifiedBy_key = v_strainmergeKey, modification_date = now()
-- WHERE _Strain_key = v_oldStrainKey
-- ;
-- UPDATE RI_RISet
-- SET _Strain_key_1 = v_newStrainKey
-- WHERE _Strain_key_1 = v_oldStrainKey
-- ;
-- UPDATE RI_RISet
-- SET _Strain_key_2 = v_newStrainKey
-- WHERE _Strain_key_2 = v_oldStrainKey
-- ;
-- -- NOTES
-- FOR v_noteTypeKey IN
-- SELECT _NoteType_key
-- FROM MGI_NoteType_Strain_View
-- LOOP
--     -- if both old and new strains have notes, concatenate old notes onto new notes
--     IF EXISTS (select 1 from MGI_Note 
-- 		WHERE _MGIType_key = 10 and _NoteType_key = v_noteTypeKey and _Object_key = v_newStrainKey)
--        AND
--        EXISTS (select 1 from MGI_Note 
-- 		WHERE _MGIType_key = 10 and _NoteType_key = v_noteTypeKey and _Object_key = v_oldStrainKey)
--     THEN
--     	v_note1 := n.note from MGI_Note n WHERE n._MGIType_key = 10 and n._NoteType_key = v_noteTypeKey and n._Object_key = v_newStrainKey;
-- 	v_note2 := n.note from MGI_Note n WHERE n._MGIType_key = 10 and n._NoteType_key = v_noteTypeKey and n._Object_key = v_oldStrainKey;
--         IF v_note1 != v_note2 
--         THEN
--                 v_note1 = v_note1 || E'\n' || v_note2;
--         END IF;
-- 	UPDATE MGI_Note
-- 	SET note = v_note1
-- 	WHERE _MGIType_key = 10 
-- 	      AND _NoteType_key = v_noteTypeKey 
-- 	      AND _Object_key = v_newStrainKey 
-- 	;
-- 	DELETE FROM MGI_Note WHERE _MGIType_key = 10 and _NoteType_key = v_noteTypeKey and _Object_key = v_oldStrainKey;
--     -- else if only old strain has notes, move old notes to new notes 
--     ELSIF NOT EXISTS (SELECT 1 FROM MGI_Note 
-- 		WHERE _MGIType_key = 10 and _NoteType_key = v_noteTypeKey and _Object_key = v_newStrainKey)
--        AND
--        EXISTS (SELECT 1 FROM MGI_Note 
-- 		WHERE _MGIType_key = 10 and _NoteType_key = v_noteTypeKey and _Object_key = v_oldStrainKey)
--     THEN
--         UPDATE MGI_Note 
--         SET _Object_key = v_newStrainKey 
--         WHERE _MGIType_key = 10 and _NoteType_key = v_noteTypeKey and _Object_key = v_oldStrainKey
-- 	;
--     END IF;
--     -- else if only new strain has notes, do nothing 
-- END LOOP;
-- -- END NOTES
-- -- STRAIN/MARKER/ALLELES
-- -- remove duplicates
-- DELETE FROM PRB_Strain_Marker p1
-- USING PRB_Strain_Marker p2
-- WHERE p1._Strain_key = v_oldStrainKey
-- AND p1._Marker_key = p2._Marker_key
-- AND p1._Allele_key = p2._Allele_key
-- AND p2._Strain_key = v_newStrainKey
-- ;
-- UPDATE PRB_Strain_Marker
-- SET _Strain_key = v_newStrainKey
-- WHERE _Strain_key = v_oldStrainKey
-- ;
-- -- TRANSLATIONS
-- -- remove duplicates 
-- DELETE FROM MGI_Translation t1
-- USING MGI_TranslationType tt, MGI_Translation t2
-- WHERE tt._MGIType_key = 10
-- and tt._TranslationType_key = t1._TranslationType_key
-- and t1._Object_key = v_oldStrainKey
-- and tt._TranslationType_key = t2._TranslationType_key
-- and t2._Object_key = v_newStrainKey
-- and t1.badName = t2.badName
-- ;
-- UPDATE MGI_Translation
-- SET _Object_key = v_newStrainKey
-- FROM MGI_TranslationType tt
-- WHERE tt._MGIType_key = 10
-- AND tt._TranslationType_key = MGI_Translation._TranslationType_key
-- AND MGI_Translation._Object_key = v_oldStrainKey
-- ;
-- -- SETS 
-- -- remove duplicates 
-- DELETE FROM MGI_SetMember s1
-- USING MGI_Set s, MGI_SetMember s2
-- WHERE s._MGIType_key = 10
-- AND s._Set_key = s1._Set_key
-- AND s1._Object_key = v_oldStrainKey
-- AND s._Set_key = s2._Set_key
-- AND s2._Object_key = v_newStrainKey
-- ;
-- UPDATE MGI_SetMember
-- SET _Object_key = v_newStrainKey
-- FROM MGI_Set s
-- WHERE s._MGIType_key = 10
-- and s._Set_key = MGI_SetMember._Set_key
-- and MGI_SetMember._Object_key = v_oldStrainKey
-- ;
-- -- SYNONYMS 
-- UPDATE MGI_Synonym
-- SET _Object_key = v_newStrainKey
-- WHERE _Object_key = v_oldStrainKey
-- AND _MGIType_key = 10
-- ;
-- -- if the strain names are not equal 
-- --   a.  make old strain name a synonym of the new strain 
-- --   b.  make old strain name a translation of the new strain 
-- IF (SELECT strain FROM PRB_Strain WHERE _Strain_key = v_oldStrainKey) !=
--    (SELECT strain FROM PRB_Strain WHERE _Strain_key = v_newStrainKey)
-- THEN
-- 	v_nextKey := nextval('mgi_synonym_seq');
-- 	--v_synTypeKey := _SynonymType_key from MGI_SynonymType_Strain_View  WHERE synonymType = 'nomenclature history';
-- 	v_synTypeKey := 1001;
-- 	IF v_nextKey IS NULL 
-- 	THEN
-- 		v_nextKey := 1000;
-- 	END IF;
-- 	INSERT INTO MGI_Synonym (_Synonym_key, _MGIType_key, _Object_key, _SynonymType_key, _Refs_key, synonym)
-- 	SELECT v_nextKey, 10, v_newStrainKey, v_synTypeKey, null, strain
-- 	FROM PRB_Strain WHERE _Strain_key = v_oldStrainKey
-- 	;
-- 	v_nextKey := max(_Translation_key) + 1 from MGI_Translation;
-- 	v_nextSeqKey := max(sequenceNum) + 1 from MGI_Translation WHERE _TranslationType_key = 1007;
-- 	INSERT INTO MGI_Translation (_Translation_key, _TranslationType_key, _Object_key, badName, sequenceNum,
-- 		_CreatedBy_key, _ModifiedBy_key, creation_date, modification_date)
-- 	SELECT v_nextKey, 1007, v_newStrainKey, strain, v_nextSeqKey, v_strainmergeKey, v_strainmergeKey,
-- 		now(), now()
-- 	FROM PRB_Strain WHERE _Strain_key = v_oldStrainKey
-- 	;
-- END IF;
-- DELETE FROM PRB_Strain WHERE _Strain_key = v_oldStrainKey;
-- END;
-- $$;


CREATE FUNCTION mgd.prb_probe_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: PRB_Probe_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INPUT:
--	none
-- RETURNS:
--	NEW
-- delete GXD_ProbePrep records if they are no longer used in Assays
-- then the system should allow the probe to be deleted
DELETE FROM GXD_ProbePrep p
WHERE p._Probe_key = OLD._Probe_key
AND NOT EXISTS (SELECT 1 FROM GXD_Assay a WHERE p._ProbePrep_key = a._ProbePrep_key)
;
DELETE FROM IMG_ImagePane_Assoc a
WHERE a._Object_key = OLD._Probe_key
AND a._MGIType_key = 3
;
DELETE FROM MAP_Coord_Feature a
WHERE a._Object_key = OLD._Probe_key
AND a._MGIType_key = 3
;
DELETE FROM MAP_Coordinate a
WHERE a._Object_key = OLD._Probe_key
AND a._MGIType_key = 3
;
DELETE FROM MGI_Note a
WHERE a._Object_key = OLD._Probe_key
AND a._MGIType_key = 3
;
DELETE FROM MGI_Property a
WHERE a._Object_key = OLD._Probe_key
AND a._MGIType_key = 3
;
DELETE FROM MGI_Reference_Assoc a
WHERE a._Object_key = OLD._Probe_key
AND a._MGIType_key = 3
;
DELETE FROM MGI_Synonym a
WHERE a._Object_key = OLD._Probe_key
AND a._MGIType_key = 3
;
DELETE FROM ACC_Accession a
WHERE a._Object_key = OLD._Probe_key
AND a._MGIType_key = 3
;
RETURN OLD;
END;
$$;


CREATE FUNCTION mgd.prb_probe_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: PRB_Probe_insert()
-- DESCRIPTOIN:
--	this insert trigger will call ACC_assignMGI
--	in order to add a distinct MGI accession id
--	to the NEW object
-- INPUT:
--	none
-- RETURNS:
--	NEW
PERFORM ACC_assignMGI(1001, NEW._Probe_key, 'Segment');
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.prb_processanonymoussource(v_msokey integer, v_segmenttypekey integer, v_vectortypekey integer, v_organismkey integer, v_strainkey integer, v_tissuekey integer, v_genderkey integer, v_celllinekey integer, v_age text, v_tissuetreatment text, v_modifiedbykey integer, v_verifyorganismedit integer DEFAULT 1, OUT v_newmsokey integer) RETURNS integer
    LANGUAGE plpgsql
    AS $$
-- process (modify) an Anonymous Molecular Source                         
--                                                                       
-- assumes:                                                             
-- 	that all edits to MS attributes are allowed.                   
--                                                                    
--      if this SP is called from the Molecular Source Processor     
--      then the MSP has already checked MGI_Attribute_History and  
--      has determined which attributes can be updated.            
--                                                                
-- does:                                                         
--                                                              
--      create a new or update an existing MSO (PRB_Source)    
--                                                            
-- implements:                                                
--     collapsing; that is, if a MSO has not been curator-edited
--     then it can be shared by multiple Probe or Sequence objects. 
--                                                                 
-- output:                                                        
--                                                               
--     the new MSO key (v_newMSOKey).                           
--                                                             
--     OR an error if the MSO being processed exists in the database 
--     and is not Anonymous.                                      
--                                                               
DECLARE
v_isCuratorEdited int;
v_ageMin numeric;
v_ageMax numeric;
BEGIN
-- if the MSO key exists in the database and is not Anonymous, then error & done.
IF EXISTS (select 1 from PRB_Source where _Source_key = v_msoKey)
   AND (select name from PRB_Source where _Source_key = v_msoKey) != null
THEN
	RAISE EXCEPTION E'Molecular Source is not Anonymous: %', v_msoKey;
	RETURN;
END IF;
-- set isCuratorEdited bit by checking if the "modified by" user is a curator (or not)
IF EXISTS (select 1 from MGI_User u, VOC_Term t
    where u._User_key = v_modifiedByKey
    and u._UserType_key = t._Term_key
    and t.term in ('Curator', 'BA', 'PI'))
THEN
        v_isCuratorEdited := 1;
ELSE
	v_isCuratorEdited := 0;
END IF;
-- if the MSO being processed is not curator-edited...    
-- then we can potentially collapse it (share it).       
IF v_isCuratorEdited = 0
THEN
    /* if an MSO exists which matches the input parameters     */
    /* and the existing MSO is not curator-edited              */
    /* then return the existing MSO key; share it (collapse).  */
    SELECT _Source_key 
	FROM PRB_Source 
	    WHERE _SegmentType_key = v_segmentTypeKey
	    AND _Vector_key = v_vectorTypeKey
	    AND _Organism_key = v_organismKey
	    AND _Strain_key = v_strainKey
	    AND _Tissue_key = v_tissueKey
	    AND _Gender_key = v_genderKey
	    AND _CellLine_key = v_cellLineKey
	    AND age = v_age
	    AND isCuratorEdited = 0
    INTO v_newMSOKey
	    ;
    IF v_newMSOKey IS NOT NULL
    THEN
    	RETURN;
    END IF;
END IF;
-- calculate the ageMin and ageMax values for age
SELECT * FROM PRB_ageMinMax(v_age) into v_ageMin, v_ageMax;
-- if the MSO key exists and if any MSO attribute has changed...
IF EXISTS (select 1 from PRB_Source where _Source_key = v_msoKey)
THEN
   v_newMSOKey := v_msoKey;
   IF (select _SegmentType_key from PRB_Source where _Source_key = v_msoKey) != v_segmentTypeKey or
      (select _Vector_key from PRB_Source where _Source_key = v_msoKey) != v_vectorTypeKey or
      (select _Organism_key from PRB_Source where _Source_key = v_msoKey) != v_organismKey or
      (select _Strain_key from PRB_Source where _Source_key = v_msoKey) != v_strainKey or
      (select _Tissue_key from PRB_Source where _Source_key = v_msoKey) != v_tissueKey or
      (select _Gender_key from PRB_Source where _Source_key = v_msoKey) != v_genderKey or
      (select _CellLine_key from PRB_Source where _Source_key = v_msoKey) != v_cellLineKey or
      (select age from PRB_Source where _Source_key = v_msoKey) != v_age or
      (select description from PRB_Source where _Source_key = v_msoKey) != v_tissueTreatment or
      ((select description from PRB_Source where _Source_key = v_msoKey) is null and (v_tissueTreatment is not null)) or
      ((select description from PRB_Source where _Source_key = v_msoKey) is not null and (v_tissueTreatment is null))
   THEN
       -- if the MSO is collapsed, then uncollapse it (create a new MSO)
       IF (select count(*) from PRB_Probe where _Source_key = v_msoKey) > 1 or
          (select count(*) from SEQ_Source_Assoc where _Source_key = v_msoKey) > 1
       THEN
	   -- insert a new MSO record using the existing collapsed MSO object
	   INSERT INTO PRB_Source
	   SELECT nextval('prb_source_seq'), _SegmentType_key, _Vector_key, _Organism_key,
	       _Strain_key, _Tissue_key, _Gender_key, _CellLine_key, _Refs_key,
	       name, description, age, ageMin, ageMax,
	       isCuratorEdited, _CreatedBy_key, _ModifiedBy_key, creation_date, modification_date
	   FROM PRB_Source 
	   WHERE _Source_key = v_msoKey
	   ;
	
	   -- update the new MSO using the input parameters
	   UPDATE PRB_Source
	   SET _SegmentType_key = v_segmentTypeKey,
	   _Vector_key = v_vectorTypeKey,
	   _Organism_key = v_organismKey,
	   _Strain_key = v_strainKey,
	   _Tissue_key = v_tissueKey,
	   _Gender_key = v_genderKey,
	   _CellLine_key = v_cellLineKey,
	   age = v_age,
	   ageMin = v_ageMin,
	   ageMax = v_ageMax,
	   description = v_tissueTreatment,
	   isCuratorEdited = v_isCuratorEdited,
	   _ModifiedBy_key = v_modifiedByKey,
	   modification_date = now()
	   WHERE _Source_key = v_newMSOKey
	   ;
	   RETURN;
       -- else, just update it
       ELSE
	   UPDATE PRB_Source
	   SET _SegmentType_key = v_segmentTypeKey,
	   _Vector_key = v_vectorTypeKey,
	   _Organism_key = v_organismKey,
	   _Strain_key = v_strainKey,
	   _Tissue_key = v_tissueKey,
	   _Gender_key = v_genderKey,
	   _CellLine_key = v_cellLineKey,
	   age = v_age,
	   ageMin = v_ageMin,
	   ageMax = v_ageMax,
	   description = v_tissueTreatment,
	   isCuratorEdited = v_isCuratorEdited,
	   _ModifiedBy_key = v_modifiedByKey,
	   modification_date = now()
	   WHERE _Source_key = v_msoKey
	   ;
	   RETURN;
       END IF;
   END IF;
-- else create a new MSO object
ELSE
	INSERT INTO PRB_Source
	VALUES (nextval('prb_source_seq'), v_segmentTypeKey, v_vectorTypeKey, v_organismKey,
	    v_strainKey, v_tissueKey, v_genderKey, v_cellLineKey, NULL, NULL, v_tissueTreatment, v_age,
	    v_ageMin, v_ageMax, v_isCuratorEdited, v_modifiedBykey, v_modifiedBykey,
	    now(), now())
	;
	RETURN;
END IF;
RETURN;
END;
$$;


CREATE FUNCTION mgd.prb_processprobesource(v_objectkey integer, v_msokey integer, v_isanon integer, v_organismkey integer, v_strainkey integer, v_tissuekey integer, v_genderkey integer, v_celllinekey integer, v_age text, v_tissuetreatment text, v_modifiedbykey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_newMSOKey int;
v_segmentTypeKey int;
v_vectorTypeKey int;
BEGIN
-- 
-- process (modify) a Probes's Molecular Source
-- 
IF v_isAnon = 1
THEN
    v_segmentTypeKey := _Term_key FROM VOC_Term WHERE _Vocab_key = 10 AND term = 'Not Specified';
    v_vectorTypeKey := _Term_key FROM VOC_Term WHERE _Vocab_key = 24 AND term = 'Not Specified';
    IF (SELECT name FROM PRB_Source WHERE _Source_key = v_msoKey) != NULL
    THEN
        -- if changing from a Named source to an Anonymous source ...
	SELECT * INTO v_newMSOKey 
	FROM PRB_processAnonymousSource(NULL, v_segmentTypeKey, v_vectorTypeKey, v_organismKey, v_strainKey,
                	v_tissueKey, v_genderKey, v_cellLineKey, v_age, v_tissueTreatment, v_modifiedByKey, 0);
	IF NOT FOUND
	THEN
        	RAISE EXCEPTION E'PRB_processProbeSource : perform PRB_processAnonymousSource failed.';
        	RETURN;
	END IF;
    ELSE
	SELECT * INTO v_newMSOKey 
	FROM PRB_processAnonymousSource(v_msoKey, v_segmentTypeKey, v_vectorTypeKey, v_organismKey, v_strainKey,
                	v_tissueKey, v_genderKey, v_cellLineKey, v_age, v_tissueTreatment, v_modifiedByKey, 0);
	IF NOT FOUND
	THEN
        	RAISE EXCEPTION E'PRB_processProbeSource : perform PRB_processAnonymousSource failed.';
        	RETURN;
	END IF;
    END IF;
ELSE
   v_newMSOKey := v_msoKey;
END IF;
-- associate the Probe with the MSO
UPDATE PRB_Probe
SET _Source_key = v_newMSOKey,
    _ModifiedBy_key = v_modifiedByKey,
    modification_date = now()
WHERE _Probe_key = v_objectKey
;
-- delete the input v_msoKey if it's now an orphan
IF NOT EXISTS (SELECT 1 FROM SEQ_Source_Assoc WHERE _Source_key = v_msoKey) AND
   NOT EXISTS (SELECT 1 FROM PRB_Probe WHERE _Source_key = v_msoKey) AND
   NOT EXISTS (SELECT 1 FROM GXD_Antigen WHERE _Source_key = v_msoKey)
THEN
    DELETE FROM PRB_Source WHERE _Source_key = v_msoKey;
END IF;
END;
$$;


CREATE FUNCTION mgd.prb_processseqloadersource(v_assockey integer, v_objectkey integer, v_msokey integer, v_organismkey integer, v_strainkey integer, v_tissuekey integer, v_genderkey integer, v_celllinekey integer, v_modifiedbykey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_isAnon int;
v_newMSOKey int;
BEGIN
-- process (modify) a Sequence's Molecular Source       
-- from the Sequence Loader                             
-- if changing old Named -> new Named, then v_msoKey = _Source_key of new Named source 
-- if changing old Anon -> new Named, then v_msoKey = _Source_key of new Named source 
-- if changing old Named -> Anonymous, then v_msoKey = _Source_key of old Named source 
-- if changing old Anon -> new Anon, then v_msoKey = _Source_key of old Anon source 
v_isAnon := 1;
v_newMSOKey := 0;
IF (SELECT name from PRB_Source WHERE _Source_key = v_msoKey) IS NOT NULL
THEN
	v_isAnon := 0;
END IF;
SELECT * FROM PRB_processSequenceSource (v_isAnon, v_assocKey, v_objectKey, v_msoKey, v_organismKey, v_strainKey,
        v_tissueKey, v_genderKey, v_cellLineKey, 'Not Resolved', v_modifiedByKey)
INTO v_newMSOKey;
RAISE NOTICE '%', v_newMSOKey;
IF NOT FOUND
THEN
	RAISE EXCEPTION E'PRB_processSeqLoaderSource : perform PRB_processSequenceSource failed.';
	RETURN;
END IF;
END;
$$;


CREATE FUNCTION mgd.prb_processsequencesource(v_isanon integer, v_assockey integer, v_objectkey integer, v_msokey integer, v_organismkey integer, v_strainkey integer, v_tissuekey integer, v_genderkey integer, v_celllinekey integer, v_age text, v_modifiedbykey integer, OUT v_newmsokey integer) RETURNS integer
    LANGUAGE plpgsql
    AS $$
DECLARE
v_newAssocKey int;
v_segmentTypeKey int;
v_vectorTypeKey int;
BEGIN
-- process (modify) a Sequence's Molecular Source  
-- 
-- if changing old Named -> new Named, then v_msoKey = _Source_key of new Named source 
-- if changing old Anon -> new Named, then v_msoKey = _Source_key of new Named source 
-- if changing old Named -> Anonymous, then v_msoKey = _Source_key of old Named source
-- if changing old Anon -> new Anon, then v_msoKey = _Source_key of old Anon source
-- process a Named Source
IF v_isAnon = 0
THEN
    -- just change the Association
    UPDATE SEQ_Source_Assoc
    SET _Source_key = v_msoKey,
    	_ModifiedBy_key = v_modifiedByKey,
    	modification_date = now()
    WHERE _Assoc_key = v_assocKey
    ;
ELSE
-- process an Anonymous Source */
    v_segmentTypeKey := _Term_key from VOC_Term where _Vocab_key = 10 and term = 'Not Applicable';
    v_vectorTypeKey := _Term_key from VOC_Term where _Vocab_key = 24 and term = 'Not Applicable';
    IF (SELECT name FROM PRB_Source WHERE _Source_key = v_msoKey) != null
    THEN
        -- if changing from a Named source to an Anonymous source ...
        SELECT * FROM PRB_processAnonymousSource(0, v_segmentTypeKey, v_vectorTypeKey, v_organismKey, v_strainKey,
                        v_tissueKey, v_genderKey, v_cellLineKey, v_age, NULL, v_modifiedByKey, 1)
	INTO v_newMSOKey
	;
        IF NOT FOUND
	THEN
	        RAISE EXCEPTION 'PRB_processSequenceSource: Could not execute PRB_processAnonymousSource';
		RETURN;
        END IF;
    ELSE
	-- modifying an existing Anonymous source
        SELECT * FROM PRB_processAnonymousSource(v_msoKey, v_segmentTypeKey, v_vectorTypeKey, v_organismKey, v_strainKey,
                        v_tissueKey, v_genderKey, v_cellLineKey, v_age, NULL, v_modifiedByKey, 1)
	INTO v_newMSOKey 
	;
        IF NOT FOUND
	THEN
	        RAISE EXCEPTION 'PRB_processSequenceSource: Could not execute PRB_processAnonymousSource';
		RETURN;
        END IF;
    END IF;
    -- associate the Sequence with the MSO
    IF EXISTS (SELECT 1 FROM SEQ_Source_Assoc WHERE _Sequence_key = v_objectKey AND _Source_key = v_msoKey)
    THEN
        UPDATE SEQ_Source_Assoc
        SET _Source_key = v_newMSOKey,
	    _ModifiedBy_key = v_modifiedByKey,
	    modification_date = now()
        WHERE _Assoc_key = v_assocKey
	;
    ELSE
        v_newAssocKey := nextval('seq_source_assoc_seq');
        INSERT INTO SEQ_Source_Assoc 
        VALUES (v_newAssocKey, v_objectKey, v_newMSOKey, v_modifiedByKey, v_modifiedByKey, now(), now())
	;
    END IF;
    
    -- delete the input v_msoKey if it's now an orphan
    
    IF NOT EXISTS (select 1 from SEQ_Source_Assoc where _Source_key = v_msoKey) and
       NOT EXISTS (select 1 from PRB_Probe where _Source_key = v_msoKey) and
       NOT EXISTS (select 1 from GXD_Antigen where _Source_key = v_msoKey)
    THEN
        DELETE FROM PRB_Source WHERE _Source_key = v_msoKey;
    END IF;
END IF;
END;
$$;


CREATE FUNCTION mgd.prb_reference_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: PRB_Reference_delete()
-- DESCRIPTOIN:
--	
-- INPUT:
--	none
-- RETURNS:
--	NEW
DELETE FROM ACC_AccessionReference ar
USING ACC_Accession a
WHERE a._Object_key = OLD._Probe_key
AND a._MGIType_key = 3
and a._Accession_key = ar._Accession_key
and ar._Refs_key = OLD._Refs_key
;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.prb_reference_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: PRB_Reference_update()
-- DESCRIPTOIN:
-- update the accession reference that is attached to this J:
-- INPUT:
--	none
-- RETURNS:
--	NEW
UPDATE ACC_AccessionReference
SET _Refs_key = NEW._Refs_key
FROM ACC_Accession a
WHERE a._Object_key = NEW._Probe_key
and a._MGIType_key = 3 
and a._Accession_key = ACC_AccessionReference._Accession_key
and ACC_AccessionReference._Refs_key = OLD._Refs_key
;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.prb_setstrainreview(v_markerkey integer DEFAULT NULL::integer, v_allelekey integer DEFAULT NULL::integer, v_reviewkey integer DEFAULT 8581446) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_annotTypeKey int;
v_qualifierKey int;
v_strainKey int;
BEGIN
-- sets the "Needs Review" flag for all strains of a marker or allele.
-- default "Needs Review - symbol" flag for all strains of a marker or allele (8581446).
-- called during nomenclature update process (MRK_simpleWithdrawal, MRK_updateKeys)
-- or when allele symbols is updated (trigger/ALL_Allele)
-- or when a phenotype mutant is withdrawn as an allele of (MRK_mergeWithdrawal)
v_annotTypeKey := 1008;
v_qualifierKey := 1614158;
IF (v_markerKey = null) AND (v_alleleKey = null)
THEN
    RAISE EXCEPTION E'Either marker or allele key must be non-null';
    RETURN;
END IF;
-- marker key is non-null
IF v_markerKey IS NOT null
THEN
    FOR v_strainKey IN
    SELECT DISTINCT _Strain_key
    FROM PRB_Strain_Marker
    WHERE _Marker_key = v_markerKey
    LOOP
	IF NOT EXISTS (SELECT 1 FROM VOC_Annot
	    WHERE _AnnotType_key = v_annotTypeKey
		AND _Object_key = v_strainKey
		AND _Term_key = v_reviewKey)
	THEN
	    INSERT INTO VOC_Annot (_Annot_key, _AnnotType_key, _Object_key, _Term_key, _Qualifier_key,
		creation_date, modification_date)
	    VALUES (nextval('voc_annot_seq'), v_annotTypeKey, v_strainKey,
	        v_reviewKey, v_qualifierKey, now(), now())
	    ;
        END IF;
    END LOOP;
-- allele key is non-null
ELSE	
    FOR v_strainKey IN
    SELECT DISTINCT _Strain_key
    FROM PRB_Strain_Marker
    WHERE _Allele_key = v_alleleKey
    LOOP
	IF NOT EXISTS (SELECT 1 FROM VOC_Annot
	    WHERE _AnnotType_key = v_annotTypeKey
		AND _Object_key = v_strainKey
		AND _Term_key = v_reviewKey)
	THEN
	    INSERT INTO VOC_Annot (_Annot_key, _AnnotType_key, _Object_key, _Term_key, _Qualifier_key, 
		creation_date, modification_date)
	    VALUES (nextval('voc_annot_seq'), v_annotTypeKey, v_strainKey,
	        v_reviewKey, v_qualifierKey, now(), now())
	    ;
        END IF;
    END LOOP;
END IF;
RETURN;
END;
$$;


CREATE FUNCTION mgd.prb_source_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: PRB_Source_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INPUT:
--	none
-- RETURNS:
--	NEW
DELETE FROM IMG_ImagePane_Assoc a
WHERE a._Object_key = OLD._Source_key
AND a._MGIType_key = 5
;
DELETE FROM MAP_Coord_Feature a
WHERE a._Object_key = OLD._Source_key
AND a._MGIType_key = 5
;
DELETE FROM MAP_Coordinate a
WHERE a._Object_key = OLD._Source_key
AND a._MGIType_key = 5
;
DELETE FROM MGI_Note a
WHERE a._Object_key = OLD._Source_key
AND a._MGIType_key = 5
;
DELETE FROM MGI_Property a
WHERE a._Object_key = OLD._Source_key
AND a._MGIType_key = 5
;
DELETE FROM MGI_Reference_Assoc a
WHERE a._Object_key = OLD._Source_key
AND a._MGIType_key = 5
;
DELETE FROM MGI_Synonym a
WHERE a._Object_key = OLD._Source_key
AND a._MGIType_key = 5
;
DELETE FROM ACC_Accession a
WHERE a._Object_key = OLD._Source_key
AND a._MGIType_key = 5
;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.prb_strain_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: PRB_Strain_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INPUT:
--	none
-- RETURNS:
--	NEW
DELETE FROM IMG_ImagePane_Assoc a
WHERE a._Object_key = OLD._Strain_key
AND a._MGIType_key = 10
;
DELETE FROM MAP_Coord_Feature a
WHERE a._Object_key = OLD._Strain_key
AND a._MGIType_key = 10
;
DELETE FROM MAP_Coordinate a
WHERE a._Object_key = OLD._Strain_key
AND a._MGIType_key = 10
;
DELETE FROM MGI_Note a
WHERE a._Object_key = OLD._Strain_key
AND a._MGIType_key = 10
;
DELETE FROM MGI_Property a
WHERE a._Object_key = OLD._Strain_key
AND a._MGIType_key = 10
;
DELETE FROM MGI_Reference_Assoc a
WHERE a._Object_key = OLD._Strain_key
AND a._MGIType_key = 10
;
DELETE FROM MGI_Synonym a
WHERE a._Object_key = OLD._Strain_key
AND a._MGIType_key = 10
;
DELETE FROM VOC_Annot a
WHERE a._Object_key = OLD._Strain_key
AND a._AnnotType_key in (1008, 1009)
;
DELETE FROM ACC_Accession a
WHERE a._Object_key = OLD._Strain_key
AND a._MGIType_key = 10
;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.prb_strain_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: PRB_Strain_insert()
-- DESCRIPTOIN:
--	this insert trigger will call ACC_assignMGI
--	in order to add a distinct MGI accession id
--	to the NEW object
-- INPUT:
--	none
-- RETURNS:
--	NEW
PERFORM ACC_assignMGI(1001, NEW._Strain_key, 'Strain');
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.seq_deletebycreatedby(v_createdby text) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_userKey int;
BEGIN
-- NAME: SEQ_deleteByCreatedBy
-- DESCRIPTION:
-- Delete Sequence objects by Created By
-- INPUT:
--      
-- v_createdBy mgi_user.login%TYPE
-- RETURNS:
--	VOID
--      
v_userKey := _User_key FROM MGI_User WHERE login = v_createdBy;
CREATE TEMP TABLE toDelete ON COMMIT DROP
AS SELECT _Object_key, accID, _LogicalDB_key
FROM ACC_Accession
WHERE _MGIType_key = 19
AND _CreatedBy_key = v_userKey
;
CREATE INDEX idx_toDelete ON toDelete(_Object_key);
-- Delete Marker associations to these Sequence objects by Created By
CREATE TEMP TABLE assocToDelete ON COMMIT DROP
AS SELECT a._Accession_key
FROM toDelete d, ACC_Accession a
WHERE d.accID = a.accID
AND d._LogicalDB_key = a._LogicalDB_key
AND a._MGIType_key = 2
AND a._CreatedBy_key = v_userKey
;
CREATE INDEX idx_assocToDelete ON assocToDelete(_Accession_key);
-- let's DELETE FROM in steps, it'll be faster
DELETE FROM MAP_Coord_Feature s
USING toDelete d
WHERE d._Object_key = s._Object_key
AND s._MGIType_key = 19
;
DELETE FROM SEQ_Coord_Cache s
USING toDelete d
WHERE d._Object_key = s._Sequence_key
;
DELETE FROM SEQ_Marker_Cache s
USING toDelete d
WHERE d._Object_key = s._Sequence_key
;
DELETE FROM SEQ_Probe_Cache  s
USING toDelete d
WHERE d._Object_key = s._Sequence_key
;
DELETE FROM SEQ_Source_Assoc s
USING toDelete d
WHERE d._Object_key = s._Sequence_key
;
DELETE FROM MGI_Reference_Assoc s
USING toDelete d
WHERE d._Object_key = s._Object_key
AND s._MGIType_key = 19
;
DELETE FROM MGI_Note s
USING toDelete d
WHERE d._Object_key = s._Object_key
AND s._MGIType_key = 19
;
DELETE FROM ACC_Accession s
USING toDelete d
WHERE d._Object_key = s._Object_key
AND s._MGIType_key = 19
;
DELETE FROM SEQ_Sequence_Raw s
USING toDelete d
WHERE d._Object_key = s._Sequence_key
;
DELETE FROM SEQ_Sequence s
USING toDelete d
WHERE d._Object_key = s._Sequence_key
;
DELETE FROM ACC_Accession s
USING assocToDelete d
WHERE d._Accession_key = s._Accession_key
;
END;
$$;


CREATE FUNCTION mgd.seq_deletedummy(v_seqkey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
IF (SELECT v.term
	FROM SEQ_Sequence_Acc_View a, SEQ_Sequence s, VOC_Term v
	WHERE a._Object_key = v_seqKey
	AND a._Object_key = s._Sequence_key
	AND s._SequenceStatus_key = v._Term_key
	AND v._Vocab_key = 20) != 'Not Loaded'
THEN
	RAISE EXCEPTION E'SEQ_deleteDummy: Cannot delete a non-Dummy Sequence';
	RETURN;
END IF;
DELETE FROM SEQ_Sequence_Raw WHERE _Sequence_key = v_seqKey;
DELETE FROM SEQ_Sequence WHERE _Sequence_key = v_seqKey;
END;
$$;


CREATE FUNCTION mgd.seq_deleteobsoletedummy() RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_userKey int;
BEGIN
-- delete obsolete dummies...those dummy Sequence records 
-- which are no longer associated with a Marker or Molecular Segment. 
-- for mouse, if it's not in either cache table, then it's potential obsolete 
-- for non-mouse, we have to look directly into the accession table 
-- select all sequences that have a status of 'Not Loaded' or 'DELETED' 
-- and a rawType of 'Not Loaded'
CREATE TEMP TABLE toDelete ON COMMIT DROP
AS SELECT s._Sequence_key
FROM SEQ_Sequence s, SEQ_Sequence_Raw r
WHERE s._SequenceStatus_key in (316343, 316345)
AND s._Sequence_key = r._Sequence_key
AND r.rawType = 'Not Loaded'
AND NOT EXISTS (SELECT 1 FROM SEQ_Marker_Cache sc WHERE s._Sequence_key = sc._Sequence_key)
AND NOT EXISTS (SELECT 1 FROM SEQ_Probe_Cache sc WHERE s._Sequence_key = sc._Sequence_key)
;
CREATE INDEX idx_key ON toDelete(_Sequence_key);
-- remove items from the deletion list if they are annotated to non-mouse Markers
DELETE FROM toDelete t
USING ACC_Accession a1, ACC_Accession a2, MRK_Marker m
WHERE t._Sequence_key = a1._Object_key
AND a1._MGIType_key = 19
AND a1.accID = a2.accID
AND a1._LogicalDB_key = a2._LogicalDB_Key
AND a2._MGIType_key = 2
AND a2._Object_key = m._Marker_key
AND m._Organism_key != 1
;
-- remove items from the deletion list if they are annotated to mouse Molecular Segments that are not cached 
-- those MolSegs with "The source of the material used to create this cDNA probe" note
DELETE FROM toDelete t
USING ACC_Accession a1, ACC_Accession a2, PRB_Probe p, PRB_Source s
WHERE t._Sequence_key = a1._Object_key
AND a1._MGIType_key = 19
AND a1.accID = a2.accID
AND a1._LogicalDB_key = a2._LogicalDB_Key
AND a2._MGIType_key = 3
AND a2._Object_key = p._Probe_key
AND p._Source_key = s._Source_key
AND s._Organism_key = 1
;
-- remove items from the deletion list if they are annotated to non-mouse Molecular Segments
DELETE FROM toDelete t
USING ACC_Accession a1, ACC_Accession a2, PRB_Probe p, PRB_Source s
WHERE t._Sequence_key = a1._Object_key
AND a1._MGIType_key = 19
AND a1.accID = a2.accID
AND a1._LogicalDB_key = a2._LogicalDB_Key
AND a2._MGIType_key = 3
AND a2._Object_key = p._Probe_key
AND p._Source_key = s._Source_key
AND s._Organism_key != 1
;
-- delete items left in the deletion list
DELETE FROM SEQ_Sequence_Raw s
USING todelete d
WHERE d._Sequence_key = s._Sequence_key
;
DELETE FROM SEQ_Sequence s
USING todelete d
WHERE d._Sequence_key = s._Sequence_key
;
END;
$$;


CREATE FUNCTION mgd.seq_merge(v_fromseqid text, v_toseqid text) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_fromSeqKey int;
v_toSeqKey int;
BEGIN
-- Merge v_fromSeqID to v_toSeqID
-- 1) Move non-duplicate Seq ID/Reference associations (MGI_Reference)
-- 2) Make non-duplicate "from" Seq IDs secondary to the "to" Sequence object
-- 3) Delete the "from" Sequence object
v_fromSeqKey := _Object_key from SEQ_Sequence_Acc_View where accID = v_fromSeqID and preferred = 1;
v_toSeqKey := _Object_key from SEQ_Sequence_Acc_View where accID = v_toSeqID and preferred = 1;
IF v_fromSeqKey IS NULL
THEN
	RAISE EXCEPTION E'SEQ_merge: Could not resolve %.', v_fromSeqID;
	RETURN;
END IF;
IF v_toSeqKey IS NULL
THEN
	RAISE EXCEPTION E'SEQ_merge: Could not resolve %.', v_toSeqID;
	RETURN;
END IF;
-- References
UPDATE MGI_Reference_Assoc s1
SET _Object_key = v_toSeqKey
WHERE s1._MGIType_key = 19
AND s1._Object_key = v_fromSeqKey
AND NOT EXISTS (SELECT 1 FROM MGI_Reference_Assoc s2
	WHERE s2._MGIType_key = 19
	AND s2._Object_key = v_toSeqKey
	AND s2._Refs_key = s1._Refs_key)
;
-- Accession IDs
UPDATE ACC_Accession a1
SET _Object_key = v_toSeqKey, preferred = 0
WHERE a1._MGIType_key = 19
AND a1._Object_key = v_fromSeqKey
AND NOT EXISTS (SELECT 1 FROM ACC_Accession a2
	WHERE a2._MGIType_key = 19
	AND a2._Object_key = v_toSeqKey
	AND a2.accID = a1.accID)
;
-- Delete merged Sequence object
-- This will delete any remaining References, Accession IDs, Source, Notes, Cache
DELETE FROM SEQ_Sequence_Raw WHERE _Sequence_key = v_fromSeqKey;
DELETE FROM SEQ_Sequence WHERE _Sequence_key = v_fromSeqKey;
END;
$$;


CREATE FUNCTION mgd.seq_sequence_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: SEQ_Sequence_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INPUT:
--	none
-- RETURNS:
--	NEW
DELETE FROM IMG_ImagePane_Assoc a
WHERE a._Object_key = OLD._Sequence_key
AND a._MGIType_key = 19
;
DELETE FROM MAP_Coord_Feature a
WHERE a._Object_key = OLD._Sequence_key
AND a._MGIType_key = 19
;
DELETE FROM MAP_Coordinate a
WHERE a._Object_key = OLD._Sequence_key
AND a._MGIType_key = 19
;
DELETE FROM MGI_Note a
WHERE a._Object_key = OLD._Sequence_key
AND a._MGIType_key = 19
;
DELETE FROM MGI_Property a
WHERE a._Object_key = OLD._Sequence_key
AND a._MGIType_key = 19
;
DELETE FROM MGI_Reference_Assoc a
WHERE a._Object_key = OLD._Sequence_key
AND a._MGIType_key = 19
;
DELETE FROM MGI_Synonym a
WHERE a._Object_key = OLD._Sequence_key
AND a._MGIType_key = 19
;
DELETE FROM ACC_Accession a
WHERE a._Object_key = OLD._Sequence_key
AND a._MGIType_key = 19
;
RETURN NEW;
END;
$$;


-- CREATE FUNCTION mgd.seq_split(v_fromseqid text, v_toseqids text) RETURNS void
--     LANGUAGE plpgsql
--     AS $$
-- DECLARE
-- v_fromSeqKey int;
-- v_splitStatusKey int;
-- v_toSeqKey int;
-- v_toAccID acc_accession.accID%TYPE;
-- v_idx text;
-- v_accID acc_accession.accID%TYPE;
-- v_logicalDB int;
-- BEGIN
-- -- Split v_fromSeqID to v_toSeqIDs
-- -- where v_toSeqIDs is a comma-separated list of Seq IDs to split the v_fromSeqID into
-- -- 1) Copy non-duplicate "from" Accession IDs to each "to" Sequence object and make them Secondary IDs
-- -- 2) Status the "from" Sequence object as "Split"
-- v_fromSeqKey := _Object_key from SEQ_Sequence_Acc_View where accID = v_fromSeqID and preferred = 1;
-- v_splitStatusKey := _Term_key from VOC_Term where _Vocab_key = 20 and term = 'SPLIT';
-- IF v_fromSeqKey IS NULL
-- THEN
-- 	RAISE EXCEPTION 'SEQ_split: Could not resolve %.', v_fromSeqID;
-- 	RETURN;
-- END IF;
-- -- delete cache entries from old sequence
-- DELETE FROM SEQ_Marker_Cache where _Sequence_key = v_fromSeqKey;
-- DELETE FROM SEQ_Probe_Cache where _Sequence_key = v_fromSeqKey;
-- -- For each new Sequence:
-- --      1. copy non-duplicate "from" Accession IDs to each "to" Sequence object and make them Secondary IDs
-- --      2. re-load SEQ_Marker_Cache
-- --      3. re-load SEQ_Probe_Cache
-- WHILE (v_toSeqIDs != NULL) LOOP
-- 	v_idx := position(',' in v_toSeqIDs);
-- 	IF v_idx > 0
-- 	THEN
-- 		v_toAccID := substring(v_toSeqIDs, 1, v_idx - 1);
-- 		v_toSeqIDs := substring(v_toSeqIDs, v_idx + 1, char_length(v_toSeqIDs));
-- 	ELSE
-- 		-- at end of list of v_toSeqIDs
-- 		v_toAccID := v_toSeqIDs;
-- 		v_toSeqIDs := NULL;
-- 	END IF;
-- 	IF v_toAccID != NULL
-- 	THEN
-- 		v_toSeqKey := _Object_key from SEQ_Sequence_Acc_View where accID = v_toAccID and preferred = 1;
-- 		IF v_toSeqKey is null
-- 		THEN
-- 			RAISE EXCEPTION E'SEQ_split: Could not resolve %.', v_toAccID;
-- 			RETURN;
-- 		END IF;
--                 -- delete cache entries from new sequence
--                 DELETE FROM SEQ_Marker_Cache where _Sequence_key = v_toSeqKey;
--                 DELETE FROM SEQ_Probe_Cache where _Sequence_key = v_toSeqKey;
-- 		-- copy all Accession IDs of v_fromSeqKey to v_toSeqKey - make them all secondary
-- 		FOR v_accID, v_logicalDB IN
-- 		SELECT a1.accID, a1._LogicalDB_key
-- 		FROM ACC_Accession a1
-- 		WHERE a1._MGIType_key = 19
-- 		AND a1._Object_key = v_fromSeqKey
-- 		AND NOT EXISTS (SELECT 1 FROM ACC_Accession a2
-- 			WHERE a2._MGIType_key = 19
-- 			AND a2._Object_key = v_toSeqKey
-- 			AND a2.accID = a1.accID)
-- 		LOOP
-- 			PERFORM ACC_insert (1001, v_toSeqKey, v_accID, v_logicalDB, 'Sequence', -1, 0, 0);
-- 		END LOOP;
-- 	END IF;
-- END LOOP;
-- -- Now make the final necessary modifications to the old Sequence object */
-- UPDATE SEQ_Sequence
-- SET _SequenceStatus_key = v_splitStatusKey, _ModifiedBy_key = 1001, modification_date = now()
-- WHERE _Sequence_key = v_fromSeqKey
-- ;
-- END;
-- $$;


CREATE FUNCTION mgd.voc_annot_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: VOC_Annot_insert()
-- DESCRIPTOIN:
-- INPUT:
--	none
-- RETURNS:
--	NEW
IF (SELECT t.isObsolete FROM VOC_Term t WHERE t._Term_key = NEW._Term_key) = 1
THEN
	RAISE EXCEPTION E'Cannot Annotate to an Obsolete Term.';
END IF;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.voc_copyannotevidencenotes(v_userkey integer, v_fromannotevidencekey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_noteKey int;
BEGIN
-- NAME: VOC_copyAnnotEvidenceNotes
-- DESCRIPTION:
--        
-- copy Annotation Evidence Notes from one Evidence record to another
-- INPUT:
--      
-- v_userKey 	          : MGI_User._User_key
-- v_fromAnnotEvidenceKey : VOC_Evidence._AnnotEvidence_key
-- RETURNS:
--	VOID
--      
v_noteKey := nextval('mgi_note_seq');
CREATE TEMP TABLE toAdd ON COMMIT DROP
AS SELECT row_number() over (order by _Note_key) as id, _Note_key, _NoteType_key, note
FROM MGI_Note
WHERE _MGIType_key = 25
AND _Object_key = v_fromAnnotEvidenceKey
;
CREATE INDEX idx1 ON toAdd(_Note_key);
INSERT INTO MGI_Note
SELECT v_noteKey + id, (select last_value from voc_evidence_seq), 25, _NoteType_key, note, v_userKey, v_userKey, now(), now()
FROM toAdd
;
DROP TABLE IF EXISTS toAdd;
END;
$$;


CREATE FUNCTION mgd.voc_evidence_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: VOC_Evidence_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INPUT:
--	none
-- RETURNS:
--	NEW
-- If there are no more Evidence records for this Annotation, then delete
-- the Annotation record as well
IF NOT EXISTS (SELECT 1 FROM VOC_Evidence where OLD._Annot_key = VOC_Evidence._Annot_key)
THEN
        DELETE FROM VOC_Annot a
	WHERE a._Annot_key = OLD._Annot_key
	;
END IF;
DELETE FROM IMG_ImagePane_Assoc a
WHERE a._Object_key = OLD._AnnotEvidence_key
AND a._MGIType_key = 25
;
DELETE FROM MAP_Coord_Feature a
WHERE a._Object_key = OLD._AnnotEvidence_key
AND a._MGIType_key = 25
;
DELETE FROM MAP_Coordinate a
WHERE a._Object_key = OLD._AnnotEvidence_key
AND a._MGIType_key = 25
;
DELETE FROM MGI_Note a
WHERE a._Object_key = OLD._AnnotEvidence_key
AND a._MGIType_key = 25
;
DELETE FROM MGI_Property a
WHERE a._Object_key = OLD._AnnotEvidence_key
AND a._MGIType_key = 25
;
DELETE FROM MGI_Reference_Assoc a
WHERE a._Object_key = OLD._AnnotEvidence_key
AND a._MGIType_key = 25
;
DELETE FROM MGI_Synonym a
WHERE a._Object_key = OLD._AnnotEvidence_key
AND a._MGIType_key = 25
;
DELETE FROM ACC_Accession a
WHERE a._Object_key = OLD._AnnotEvidence_key
AND a._MGIType_key = 25
;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.voc_evidence_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
v_termKey int;
v_objectKey int;
v_dagKey int;
BEGIN
-- NAME: VOC_Evidence_insert()
-- DESCRIPTOIN:
--	1) if AP/GO annotation 
--              if Reference exists as group = 'AP', 'GO'
--		in BIB_Workflow_Status,
--	   then
--		set existing BIB_Workflow_Status.isCurrent = 0
--		add new BIB_Workflow_Status._Status_key = 'Full-coded'
-- changes to this trigger require changes to procedure/BIB_updateWFStatusAP_create.object
-- changes to this trigger require changes to procedure/BIB_updateWFStatusGO_create.object
-- INPUT:
--	none
-- RETURNS:
--	NEW
v_termKey := _Term_key FROM VOC_Annot a WHERE a._Annot_key = NEW._Annot_key;
v_objectKey := _Object_key FROM VOC_Annot a WHERE a._Annot_key = NEW._Annot_key;
v_dagKey := distinct _DAG_key FROM VOC_Annot_View a, DAG_Node_View d
        WHERE a._Annot_key = NEW._Annot_key
        AND a._Vocab_key = d._Vocab_key
        AND a._Term_key = d._Object_key;
-- TR 7865: Disallow Annotation if adding an Annotation to an "unknown" term (120, 1098, 6113)
-- and an Annotation to a known Term in the same DAG already exists
-- If the Annotation to a known Term is to
-- J:72247 (InterPro)
-- J:60000 (Swiss-Prot)
-- J:72245 (Swiss-Prot)
-- J:80000 (RIKEN)
-- J:56000
-- then it's okay 
IF v_termKey in (120, 1098, 6113)
THEN
        -- determine if the same Object contains an Annotation to a different Term
        -- within the same DAG
        IF EXISTS (SELECT 1 FROM VOC_Evidence e, VOC_Annot a, VOC_AnnotType ap, VOC_VocabDAG v, DAG_Node n
                WHERE e._Refs_key not in (73199, 61933, 80961, 59154, 73197)
                AND e._Annot_key = a._Annot_key
                AND a._Object_key = v_objectKey
                AND a._Term_key != v_termKey
                AND a._AnnotType_key = ap._AnnotType_key
                AND ap._Vocab_key = v._Vocab_key
                AND v._DAG_key = v_dagKey
                AND v._DAG_key = n._DAG_key
                AND a._Term_key = n._Object_key)
        THEN
                RAISE EXCEPTION E'This Object has already been annotated to this DAG.\nTherefore, you cannot annotate it
 to an unknown term.';
        END IF;
END IF;
-- If the new Object is already annotated to an unknown term in the same DAG
-- then delete the Annotation to the unknown term.
IF v_termKey not in (120, 1098, 6113)
THEN
        -- determine if the same Object contains an Annotation to an unknown Term
        -- within the same DAG 
        IF EXISTS (SELECT 1
                FROM VOC_Evidence e, VOC_Annot a, VOC_AnnotType ap, VOC_VocabDAG v, DAG_Node n
		WHERE e._Annot_key = a._Annot_key
                AND a._Object_key = v_objectKey
                AND a._Term_key in (120, 1098, 6113)
                AND a._Term_key != v_termKey
                AND a._AnnotType_key = ap._AnnotType_key
                AND ap._Vocab_key = v._Vocab_key
                AND v._DAG_key = v_dagKey
                AND v._DAG_key = n._DAG_key
                AND a._Term_key = n._Object_key)
        THEN
                DELETE FROM VOC_Evidence 
		USING VOC_Annot a, VOC_AnnotType ap, VOC_VocabDAG v, DAG_Node n
		WHERE VOC_Evidence._Annot_key = a._Annot_key
                AND a._Object_key = v_objectKey
                AND a._Term_key != v_termKey
                AND a._Term_key in (120, 1098, 6113)
                AND a._AnnotType_key = ap._AnnotType_key
                AND ap._Vocab_key = v._Vocab_key
                AND v._DAG_key = v_dagKey
                AND v._DAG_key = n._DAG_key
                AND a._Term_key = n._Object_key
		;
        END IF;
END IF;
-- if this is a GO annotation, then remove the NOGO designation for the reference
IF (SELECT a._AnnotType_key FROM VOC_Annot a where NEW._Annot_key = a._Annot_key) = 1000
THEN
        IF EXISTS (SELECT 1 FROM BIB_Workflow_Status b
                WHERE NEW._Refs_key = b._Refs_key
                AND b._Group_key = 31576666
		AND b._Status_key != 31576674
		AND b.isCurrent = 1)
        THEN
            UPDATE BIB_Workflow_Status w set isCurrent = 0
            WHERE NEW._Refs_key = w._Refs_key
                AND w._Group_key = 31576666
            ;   
            INSERT INTO BIB_Workflow_Status 
            VALUES((select nextval('bib_workflow_status_seq')), NEW._Refs_key, 31576666, 31576674, 1, 
    	        NEW._CreatedBy_key, NEW._ModifiedBy_key, now(), now())
            ;
            PERFORM BIB_keepWFRelevance (NEW._Refs_key, NEW._CreatedBy_key);
        END IF;
END IF;
-- if this an MP annotation                       
-- and the Reference is associated with an Allele 
-- then change the Allele Reference to "Used-FC" 
IF (SELECT a._AnnotType_key from VOC_Annot a where NEW._Annot_key = a._Annot_key) = 1002
   AND EXISTS (SELECT 1 FROM VOC_Annot a, GXD_AlleleGenotype g, MGI_Reference_Assoc r, MGI_RefAssocType rt
       WHERE NEW._Annot_key = a._Annot_key
       AND a._Object_key = g._Genotype_key
       AND g._Allele_key = r._Object_key
       AND NEW._Refs_key = r._Refs_key
       AND r._MGIType_key = 11
       AND r._RefAssocType_key = rt._RefAssocType_key
       AND rt.assocType != 'Used-FC')
THEN
    -- Used-FC
    PERFORM MGI_updateReferenceAssoc (NEW._CreatedBy_key, 12, v_objectKey, NEW._Refs_key, 1017, null);
END IF;
-- if this is an MP annotation
-- any time a J# (Reference) is attached to a new/existing MP term,
-- set the Group AP/Status = "Full-coded"
IF (SELECT a._AnnotType_key FROM VOC_Annot a where NEW._Annot_key = a._Annot_key) = 1002
THEN
        IF EXISTS (SELECT 1 FROM BIB_Workflow_Status b
                WHERE NEW._Refs_key = b._Refs_key
                AND b._Group_key = 31576664
		AND b._Status_key != 31576674
                AND b.isCurrent = 1)
        THEN
            UPDATE BIB_Workflow_Status w set isCurrent = 0
            WHERE NEW._Refs_key = w._Refs_key
                AND w._Group_key = 31576664
            ;
            INSERT INTO BIB_Workflow_Status
            VALUES((select nextval('bib_workflow_status_seq')), NEW._Refs_key, 31576664, 31576674, 1,
                NEW._CreatedBy_key, NEW._ModifiedBy_key, now(), now())
            ;
        END IF;
END IF;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.voc_evidence_property_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: VOC_Evidence_Property_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INPUT:
--	none
-- RETURNS:
--	NEW
DELETE FROM MGI_Note a
WHERE a._Object_key = OLD._EvidenceProperty_key
AND a._MGIType_key = 41
;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.voc_evidence_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: VOC_Evidence_update()
-- DESCRIPTOIN:
--	
-- after update, if there are no more Evidence records to the Annotation, 
--	then delete the Annotation record as well
-- INPUT:
--	none
-- RETURNS:
--	NEW
IF NOT EXISTS (SELECT 1 FROM VOC_Evidence where OLD._Annot_key = VOC_Evidence._Annot_key)
THEN
        DELETE FROM VOC_Annot a
	WHERE a._Annot_key = OLD._Annot_key
	;
END IF;
RETURN NEW;
END;
$$;


CREATE FUNCTION mgd.voc_mergeannotations(annottypekey integer, oldkey integer, newkey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: VOC_mergeAnnotations
-- DESCRIPTION:
--        
-- Executed from MRK_updateKeys during merge process 
-- A unique VOC_Annotation record is defined by: 
--	_AnnotType_key	
--	_Object_key	
--	_Term_key	
--	_Qualifier_key	
-- A unique VOC_Evidence record is defined by: 
--	_Annot_key		
--	_EvidenceTerm_key	
--	_Refs_key		
-- Compare Annotations to oldKey to Annotations to newKey 
-- old set = annotations to oldKey                        
-- new set = annotations to newKey                        
--                                                        
-- If:                                                    
--    1) for records in old set but not in new set -> add to new set 
--    2) for records not in old set and in new set -> do nothing     
--    3) for records in old set and in new set     -> merge          
--                                                                   
-- So, what we're interested in doing is to move old set annotations 
-- into the new set and old set evidences into the new set           
-- without duplications. 
-- for annotations of the old set which are not present in the new set 
-- move the annotations to the newKey 
-- INPUT:
--      
-- annotTypeKey : VOC_Annot._AnnotType_key
-- oldKey       : VOC_Annot._Annot_key "from" key
-- newKey       : VOC_Annot._Annot_key "to" key
-- RETURNS:
--	VOID
--      
UPDATE VOC_Annot
SET _Object_key = newKey
WHERE _AnnotType_key = annotTypeKey
AND _Object_key = oldKey
AND NOT EXISTS (SELECT 1 FROM VOC_Annot a2
	WHERE a2._AnnotType_key = annotTypeKey
	AND a2._Object_key = newKey
	AND VOC_Annot._Term_key = a2._Term_key
	AND VOC_Annot._Qualifier_key = a2._Qualifier_key)
;
-- for annotations of the old set which are present in the new set 
-- and for evidences of the old set which are not present in the new set 
-- move the evidences to the new set annotation key 
UPDATE VOC_Evidence
SET _Annot_key = a2._Annot_key
FROM VOC_Annot a, VOC_Annot a2
WHERE a._AnnotType_key = annotTypeKey
AND a._Object_key = oldKey
AND a._Annot_key = VOC_Evidence._Annot_key
AND a2._AnnotType_key = annotTypeKey
AND a2._Object_key = newKey
AND a._Term_key = a2._Term_key
AND a._Qualifier_key = a2._Qualifier_key
AND not exists (SELECT 1 FROM VOC_Evidence e2
	WHERE a2._Annot_key = e2._Annot_key
	AND VOC_Evidence._EvidenceTerm_key = e2._EvidenceTerm_key
	AND VOC_Evidence._Refs_key = e2._Refs_key)
;
-- now delete all annotations to the old key that we haven't moved to the new key 
DELETE FROM VOC_Annot WHERE _AnnotType_key = annotTypeKey AND _Object_key = oldKey
;
-- TR 5425 
-- TR 10686: (122, 1114, 6115) changed to (120, 1098, 6113) 
-- now delete all annotations to unknown terms 
-- where there exists a non-IEA annotation to a known term 
-- in the same DAG 
DELETE
FROM VOC_Annot
USING VOC_AnnotType ap1, VOC_VocabDAG v1, DAG_Node n1
WHERE VOC_Annot._Object_key = newKey
AND VOC_Annot._Term_key in (120, 1098, 6113)
AND VOC_Annot._AnnotType_key = annotTypeKey
AND VOC_Annot._AnnotType_key = ap1._AnnotType_key
AND ap1._Vocab_key = v1._Vocab_key
AND v1._DAG_key = n1._DAG_key
AND VOC_Annot._Term_key = n1._Object_key
AND EXISTS (SELECT 1 FROM VOC_Annot a2, VOC_Evidence e2, VOC_AnnotType ap2, VOC_VocabDAG v2, DAG_Node n2
	WHERE VOC_Annot._Object_key = a2._Object_key
	AND VOC_Annot._AnnotType_key = a2._AnnotType_key
	AND a2._Term_key not in (120, 1098, 6113)
	AND a2._Annot_key = e2._Annot_key
	AND e2._EvidenceTerm_key != 115
	AND a2._AnnotType_key = ap2._AnnotType_key
	AND ap2._Vocab_key = v2._Vocab_key
	AND v2._DAG_key = n2._DAG_key
	AND a2._Term_key = n2._Object_key
	AND v1._DAG_key = v2._DAG_key
	AND n1._DAG_key = n2._DAG_key)
;
END;
$$;


CREATE FUNCTION mgd.voc_mergedupannotations(v_annottypekey integer, v_in_objectkey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_annotKey int;
v_objectKey int;
v_termKey int;
v_qualifierKey int;
v_annotevidenceKey int;
vv_annotKey int;
vv_objectKey int;
vv_termKey int;
vv_qualifierKey int;
vv_annotevidenceKey int;
vv_currentObjectKey int;
vv_prevObjectKey int := 0;
vv_goodAnnotKey int;
vv_badAnnotKey int;
BEGIN
-- NAME: VOC_mergeDupAnnotations
-- DESCRIPTION:
--        
-- Executed from API/AnotationService/process()
-- A unique VOC_Annotation record is defined by: 
--	_AnnotType_key	
--	_Object_key	
--	_Term_key	
--	_Qualifier_key	
-- For given _Object_key, find any duplicate VOC_Annot rows and:
-- move VOC_Evidence._Annot_key from bad/duplidate VOC_Annot record to "good" VOC_Annot record
-- delete duplicate VOC_Annot record
-- INPUT:
--      
-- annotTypeKey : VOC_Annot._AnnotType_key
-- in_objectKey    : VOC_Annot._Object_key
-- RETURNS:
--	VOID
--      
FOR v_annotTypeKey, v_objectKey, v_termKey, v_qualifierKey IN
select _annottype_key, _object_key, _term_key, _qualifier_key
from voc_annot 
where _annottype_key = v_annotTypeKey
and _object_key = v_in_objectKey
group by _annottype_key, _object_key, _term_key, _qualifier_key having count(*) > 1 
LOOP
	RAISE NOTICE 'LOOP1: _annottype_key = %, _object_key = %, _term_key = %, _qualifier_key = %', v_annotTypeKey, v_objectKey, v_termKey, v_qualifierKey;
	vv_prevObjectKey := 0;
	FOR vv_annotKey, vv_objectKey, vv_termKey, vv_qualifierKey, vv_annotevidenceKey IN
	select a._annot_key, a._object_key, a._term_key, a._qualifier_key, e._annotevidence_key
	from voc_annot a, voc_evidence e
	where a._annottype_key = v_annotTypeKey
	and a._object_key = v_objectKey
	and a._term_key =  v_termKey
	and a._qualifier_key = v_qualifierKey
	and a._annot_key = e._annot_key
	order by a._object_key, a._term_key, a._annot_key
	LOOP
		RAISE NOTICE 'LOOP2/with evidence: _annot_key = %, _object_key = %, _term_key = %, _qualifier_key = %', vv_annotKey, vv_objectKey, vv_termKey, vv_qualifierKey;
		vv_currentObjectKey := v_in_objectKey;
		IF vv_prevObjectKey != vv_currentObjectKey
		THEN
			vv_goodAnnotKey := vv_annotKey;
			vv_prevObjectKey := vv_currentObjectKey;
		ELSE
			vv_badAnnotKey := vv_annotKey;
	
			IF vv_goodAnnotKey != vv_badAnnotKey
			THEN
				RAISE NOTICE 'VOC_Evidence._AnnotEvidence_key=% from _Annot_key=% to _Annot_key=%', vv_annotevidenceKey, vv_badAnnotKey, vv_goodAnnotKey;
				RAISE NOTICE 'delete VOC_Annot._Annot_key =%', vv_badAnnotKey;
				UPDATE VOC_Evidence set _annot_key = vv_goodAnnotKey where _annotevidence_key = vv_annotevidenceKey;
				DELETE FROM VOC_Annot where _Annot_key = vv_badAnnotKey;
			END IF;
		END IF;
	END LOOP;
	vv_prevObjectKey := 0;
	FOR vv_annotKey, vv_objectKey, vv_termKey, vv_qualifierKey, vv_annotevidenceKey IN
	select a._annot_key, a._object_key, a._term_key, a._qualifier_key
	from voc_annot a
	where a._annottype_key = v_annotTypeKey
	and a._object_key = v_objectKey
	and a._term_key =  v_termKey
	and a._qualifier_key = v_qualifierKey
        and not exists (select 1 from voc_evidence e where a._annot_key = e._annot_key)
	order by a._object_key, a._term_key, a._annot_key
	LOOP
		RAISE NOTICE 'LOOP3/no evidence: _annot_key = %, _object_key = %, _term_key = %, _qualifier_key = %', vv_annotKey, vv_objectKey, vv_termKey, vv_qualifierKey;
		vv_currentObjectKey := v_in_objectKey;
		IF vv_prevObjectKey != vv_currentObjectKey
		THEN
			vv_goodAnnotKey := vv_annotKey;
			vv_prevObjectKey := vv_currentObjectKey;
		ELSE
			vv_badAnnotKey := vv_annotKey;
	
			IF vv_goodAnnotKey != vv_badAnnotKey
			THEN
				RAISE NOTICE 'delete VOC_Annot._Annot_key =%', vv_badAnnotKey;
				DELETE FROM VOC_Annot where _Annot_key = vv_badAnnotKey;
			END IF;
		END IF;
	END LOOP;
END LOOP;
END;
$$;


CREATE FUNCTION mgd.voc_mergeterms(oldkey integer, newkey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: VOC_mergeTerms
-- DESCRIPTION:
--        
-- To merge a term:
-- a.  make the accID of the old term a secondary accID of the new term 
-- b.  move annotations of the old term to the new term (avoid duplicates)
-- c.  delete the old term                                               
-- a.  make the accID of the old term a secondary accID of the new term 
-- INPUT:
--      
-- oldKey : VOC_Term._Term_key : old
-- newKey : VOC_Term._Term_key : new
-- RETURNS:
--      
UPDATE ACC_Accession
SET _Object_key = newKey, preferred = 0
WHERE _Object_key = oldKey
AND _MGIType_key = 13
;
-- b.  move annotations of the old term to the new term (avoid duplicates)
UPDATE VOC_Annot a
SET _Term_key = newKey
WHERE a._Term_key = oldKey
AND NOT EXISTS (SELECT 1 FROM VOC_Annot a2
	WHERE a2._Term_key = newKey
	AND a._AnnotType_key = a2._AnnotType_key
	AND a._Object_key = a2._Object_key
	AND a._Qualifier_key = a2._Qualifier_key)
;
-- move any evidences of the old term to the new term
-- for annotations of the old set which are present in the new set 
-- and for evidences of the old set which are not present in the new set 
-- move the evidences to the new set annotation key 
-- this moves the VOC_Evidence record to the new _Annot_key which essentially 
-- moves the Evidence record to the new term 
UPDATE VOC_Evidence e
SET _Annot_key = a2._Annot_key
FROM VOC_Annot a, VOC_Annot a2
WHERE a._Term_key = oldKey
AND a._Annot_key = e._Annot_key
AND a2._Term_key = newKey
AND a._Object_key = a2._Object_key
AND a._Qualifier_key = a2._Qualifier_key
AND NOT EXISTS (SELECT 1 FROM VOC_Evidence e2
	WHERE a2._Annot_key = e2._Annot_key
	AND e._EvidenceTerm_key = e2._EvidenceTerm_key
	AND e._Refs_key = e2._Refs_key)
;
-- now delete all annotations to the old key that we haven't moved to the new key 
DELETE FROM VOC_Annot WHERE _Term_key = oldKey;
-- c.  delete the old term (this will delete the DAG_Node as well)        
DELETE FROM VOC_Term WHERE _Term_key = oldKey;
END;
$$;


CREATE FUNCTION mgd.voc_processannotheader(v_userkey integer, v_annottypekey integer, v_objectkey integer, v_reorder smallint DEFAULT 1) RETURNS void
    LANGUAGE plpgsql
    AS $$
-- generate set of Header Terms based on Annotations of object/mgi type
-- test data: 1002, 11216
DECLARE
v_headerLabel int;
v_maxSeq int;
v_pkey int;	-- primary key of records to update
v_oldSeq int;	-- current sequence number
v_newSeq int;	-- new sequence number
BEGIN
-- NAME: VOC_processAnnotHeader
-- DESCRIPTION:
--        
-- set headers by Annotation Type, Object
-- INPUT:
--      
-- v_userKey      : MGI_User._User_key
-- v_annotTypeKey : VOC_Annot._AnnotType_key
-- v_objectKey    : VOC_Annot._Object_key
-- v_reorder      : DEFAULT 1, if updating the cache for a single object, 
--	            then re-set the sequence numbers so there are no gaps
-- RETURNS:
--	VOID
v_headerLabel := _Label_key FROM DAG_Label WHERE label = 'Header';
v_maxSeq := 0;
-- if no annotations exist, delete all headers and return
IF (SELECT count(*) FROM VOC_Annot 
	WHERE _AnnotType_key = v_annotTypeKey
	AND _Object_key = v_objectKey) = 0
THEN
	DELETE 
	FROM VOC_AnnotHeader
	WHERE _AnnotType_key = v_annotTypeKey
	AND _Object_key = v_objectKey
	;
	RETURN;
END IF;
-- set all headers to not-normal
UPDATE VOC_AnnotHeader
SET isNormal = 0
WHERE _AnnotType_key = v_annotTypeKey
AND _Object_key = v_objectKey
;
-- set of "new" headers based on most recent annotation update 
-- need to check if any ancestors are header terms 
-- and if the annotated term is itself a header term 
CREATE TEMP TABLE set0 ON COMMIT DROP
AS (SELECT DISTINCT h._Term_key, h.sequenceNum, a._Qualifier_key, 0 as isNormal
FROM VOC_Annot a, VOC_Term t, VOC_VocabDAG vd, DAG_Node d, DAG_Closure dc, DAG_Node dh, VOC_Term h
WHERE a._AnnotType_key = v_annotTypeKey
AND a._Object_key = v_objectKey
AND a._Term_key = t._Term_key
AND t._Vocab_key = vd._Vocab_key
AND vd._DAG_key = d._DAG_key
AND t._Term_key = d._Object_key
AND d._Node_key = dc._Descendent_key
AND dc._Ancestor_key = dh._Node_key
AND dh._Label_key = v_headerLabel
AND dh._Object_key = h._Term_key
UNION
SELECT DISTINCT h._Term_key, h.sequenceNum, a._Qualifier_key, 0 as isNormal
FROM VOC_Annot a, VOC_Term t, VOC_VocabDAG vd, DAG_Node d, DAG_Closure dc, DAG_Node dh, VOC_Term h
WHERE a._AnnotType_key = v_annotTypeKey
AND a._Object_key = v_objectKey
AND a._Term_key = t._Term_key
AND t._Vocab_key = vd._Vocab_key
AND vd._DAG_key = d._DAG_key
AND t._Term_key = d._Object_key
AND d._Node_key = dc._Descendent_key
AND dc._Descendent_key = dh._Node_key
AND dh._Label_key = v_headerLabel
AND dh._Object_key = h._Term_key
)
ORDER BY sequenceNum
;
-- set isNormal
-- isNormal = 1 if all of the qualifiers for a given header term = 2181424
-- else isNormal = 0
UPDATE set0 s1
SET isNormal = 1
WHERE s1._Qualifier_key = 2181424
AND NOT EXISTS (SELECT 1 FROM set0 s2 WHERE s1._Term_key = s2._Term_key AND s2._Qualifier_key != 2181424)
;
-- now SELECT the DISTINCT headers
CREATE TEMP TABLE set1 ON COMMIT DROP
AS SELECT DISTINCT _Term_key, sequenceNum, isNormal
FROM set0
;
-- set of headers that are currently cached
CREATE TEMP TABLE set2 ON COMMIT DROP
AS SELECT _AnnotHeader_key, _Term_key, sequenceNum
FROM VOC_AnnotHeader 
WHERE _AnnotType_key = v_annotTypeKey AND _Object_key = v_objectKey
ORDER BY sequenceNum
;
-- any headers in set2 that is not in set1 are deleted
DELETE 
FROM VOC_AnnotHeader
USING set1, set2
WHERE set2._AnnotHeader_key = VOC_AnnotHeader._AnnotHeader_key
AND NOT EXISTS (SELECT 1 FROM set1 WHERE set2._Term_key = set1._Term_key)
;
-- set of headers that are currently cached after deletion
CREATE TEMP TABLE set3 ON COMMIT DROP
AS SELECT _Term_key, sequenceNum
FROM VOC_AnnotHeader 
WHERE _AnnotType_key = v_annotTypeKey AND _Object_key = v_objectKey
ORDER BY sequenceNum
;
-- any headers in set1 that are not in set3 are added
CREATE TEMP TABLE toAdd ON COMMIT DROP
AS SELECT row_number() over (order by s1.sequenceNum) as id, _Term_key, sequenceNum, isNormal
FROM set1 s1 
WHERE not exists (SELECT 1 FROM set3 s3 WHERE s1._Term_key = s3._Term_key)
ORDER BY s1.sequenceNum
;
-- update the isNormal for any headers in set1 that are in set3 (existing headers)
UPDATE VOC_AnnotHeader
SET isNormal = s1.isNormal
FROM set1 s1, set3 s3
WHERE VOC_AnnotHeader._AnnotType_key = v_annotTypeKey 
AND VOC_AnnotHeader._Object_key = v_objectKey
AND VOC_AnnotHeader._Term_key = s1._Term_key
AND s1._Term_key = s3._Term_key
;
INSERT INTO VOC_AnnotHeader 
SELECT nextval('voc_annotheader_seq'), v_annotTypeKey, v_objectKey, _Term_key, v_maxSeq + id, isNormal,
	v_userKey, v_userKey, NULL, NULL, now(), now()
FROM toAdd
;
-- if there is only one Annotation header in the cache, the automatically approve it
IF (SELECT count(*) FROM VOC_AnnotHeader WHERE _AnnotType_key = v_annotTypeKey AND _Object_key = v_objectKey) = 1
THEN
    UPDATE VOC_AnnotHeader
    SET _ApprovedBy_key = v_userKey, approval_date = now()
    WHERE _AnnotType_key = v_annotTypeKey AND _Object_key = v_objectKey
    ;
-- else if there is at least one new header added to the cache, then set all headers to non-approved
ELSIF (SELECT count(*) FROM toAdd) >= 1
THEN
    UPDATE VOC_AnnotHeader
    SET _ApprovedBy_key = null, approval_date = null
    WHERE _AnnotType_key = v_annotTypeKey AND _Object_key = v_objectKey
    ;
END IF;
-- if we're updating the cache for a single object, then re-set the sequence numbers so there are no gaps
IF (v_reorder = 1)
THEN
    v_newSeq := 1;
    FOR v_pkey, v_oldSeq IN
    SELECT _AnnotHeader_key, sequenceNum
    FROM VOC_AnnotHeader
    WHERE _AnnotType_key = v_annotTypeKey 
    AND _Object_key = v_objectKey
    ORDER BY sequenceNum
    LOOP
        UPDATE VOC_AnnotHeader SET sequenceNum = v_newSeq WHERE _AnnotHeader_key = v_pkey;
        v_newSeq := v_newSeq + 1;
    END LOOP;
END IF;
DROP TABLE IF EXISTS set0;
DROP TABLE IF EXISTS set1;
DROP TABLE IF EXISTS set2;
DROP TABLE IF EXISTS set3;
DROP TABLE IF EXISTS toAdd;
END;
$$;


CREATE FUNCTION mgd.voc_processannotheaderall(v_annottypekey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_headerLabel int;
v_pkey int;
v_objectKey int;
v_termKey int;
v_isNormal smallint;
v_oldSeq int;	-- current sequence number
v_newSeq int;	-- new sequence number
v_prevObjectKey int;
rec record;
BEGIN
-- NAME: VOC_processAnnotHeaderAll
-- DESCRIPTION:
--        
-- incrementally update VOC_AnnotHeader by annotation type
-- INPUT:
--      
-- v_annotTypeKey : VOC_Annot._AnnotType_key
-- RETURNS:
--	VOID
--      
v_headerLabel := _Label_key FROM DAG_Label WHERE label = 'Header';
-- set of 'new' headers based on most recent annotation update 
-- need to check if any ancestors are header terms
-- AND if the annotated term is itself a header term 
CREATE TEMP TABLE set0 ON COMMIT DROP 
AS SELECT DISTINCT a._Object_key, h._Term_key, h.sequenceNum, a._Qualifier_key, 0 as isNormal
FROM VOC_Annot a, VOC_Term t, VOC_VocabDAG vd, DAG_Node d, DAG_Closure dc, DAG_Node dh, VOC_Term h
WHERE a._AnnotType_key = v_annotTypeKey
AND a._Term_key = t._Term_key
AND t._Vocab_key = vd._Vocab_key
AND vd._DAG_key = d._DAG_key
AND t._Term_key = d._Object_key
AND d._Node_key = dc._Descendent_key
AND dc._Ancestor_key = dh._Node_key
AND dh._Label_key = v_headerLabel
AND dh._Object_key = h._Term_key
UNION
SELECT DISTINCT a._Object_key, h._Term_key, h.sequenceNum, a._Qualifier_key, 0 as isNormal
FROM VOC_Annot a, VOC_Term t, VOC_VocabDAG vd, DAG_Node d, DAG_Closure dc, DAG_Node dh, VOC_Term h
WHERE a._AnnotType_key = v_annotTypeKey
AND a._Term_key = t._Term_key
AND t._Vocab_key = vd._Vocab_key
AND vd._DAG_key = d._DAG_key
AND t._Term_key = d._Object_key
AND d._Node_key = dc._Descendent_key
AND dc._Descendent_key = dh._Node_key
AND dh._Label_key = v_headerLabel
AND dh._Object_key = h._Term_key
ORDER BY _Object_key, sequenceNum
;
CREATE INDEX set0_idx1 ON set0(_Term_key);
CREATE INDEX set0_idx2 ON set0(_Object_key);
CREATE INDEX set0_idx3 ON set0(_Qualifier_key);
-- set isNormal 
-- isNormal = 1 if all of the qualifiers for a given header term = 2181424 
-- else isNormal = 0 
CREATE TEMP TABLE normal ON COMMIT DROP
AS SELECT DISTINCT _Object_key, _Term_key
FROM set0 s1
WHERE s1._Qualifier_key = 2181424
AND NOT EXISTS (SELECT 1 FROM set0 s2
WHERE s1._Object_key = s2._Object_key
AND s1._Term_key = s2._Term_key
AND s2._Qualifier_key != 2181424)
;
UPDATE set0
SET isNormal = 1
FROM normal n
WHERE n._Object_key = set0._Object_key
AND n._Term_key = set0._Term_key
;
-- now SELECT the DISTINCT headers 
CREATE TEMP TABLE set1 ON COMMIT DROP
AS SELECT DISTINCT _Object_key, _Term_key, sequenceNum, isNormal
FROM set0
;
CREATE INDEX set1_idx1 ON set1(_Term_key);
CREATE INDEX set1_idx2 ON set1(_Object_key);
-- set of headers that are currently cached 
CREATE TEMP TABLE set2 ON COMMIT DROP
AS SELECT _AnnotHeader_key, _Object_key, _Term_key, sequenceNum, isNormal
FROM VOC_AnnotHeader
WHERE _AnnotType_key = v_annotTypeKey
ORDER BY _Object_key, sequenceNum
;
CREATE INDEX set2_idx1 ON set2(_Term_key);
CREATE INDEX set2_idx2 ON set2(_Object_key);
-- any headers in set2 that is not in set1 are deleted 
CREATE TEMP TABLE toDelete ON COMMIT DROP
AS SELECT s2._AnnotHeader_key 
FROM set2 s2
WHERE not EXISTS
(SELECT 1 FROM set1 s1 WHERE s2._Term_key = s1._Term_key AND s2._Object_key = s1._Object_key)
-- if VOC_AnnotHeader exists for _object_key that is not in VOC_Annot._Annottype_key = v_annottype, then delete
UNION
SELECT h._AnnotHeader_key
FROM VOC_AnnotHeader h
WHERE NOT EXISTS (SELECT 1 FROM VOC_Annot g WHERE g._AnnotType_key = v_annotTypeKey and h._object_key = g._object_key)
;
CREATE INDEX toDelete_idx1 ON toDelete(_AnnotHeader_key);
delete FROM VOC_AnnotHeader
using toDelete d
WHERE d._AnnotHeader_key = VOC_AnnotHeader._AnnotHeader_key
;
-- set of headers that are currently cached after deletion 
CREATE TEMP TABLE set3 ON COMMIT DROP
AS SELECT _Object_key, _Term_key, sequenceNum
FROM VOC_AnnotHeader
WHERE _AnnotType_key = v_annotTypeKey
ORDER BY _Object_key, sequenceNum
;
CREATE INDEX set3_idx1 ON set3(_Term_key);
CREATE INDEX set3_idx2 ON set3(_Object_key);
-- any headers in set1 that are not in set3 are added 
CREATE TEMP TABLE toAdd ON COMMIT DROP
AS SELECT 1 as SERIAL, _Object_key, _Term_key, sequenceNum, isNormal
FROM set1 s1
WHERE NOT EXISTS (SELECT 1 FROM set3 s3 WHERE s1._Term_key = s3._Term_key AND s1._Object_key = s3._Object_key)
ORDER BY s1._Object_key, s1.sequenceNum
;
-- update the isNormal bit for any headers in #set1 that are in #set3 (existing headers) 
update VOC_AnnotHeader
set isNormal = s1.isNormal
FROM set1 s1, set3 s3
WHERE VOC_AnnotHeader._AnnotType_key = v_annotTypeKey
AND VOC_AnnotHeader._Object_key = s1._Object_key
AND VOC_AnnotHeader._Term_key = s1._Term_key
AND s1._Object_key = s3._Object_key
AND s1._Term_key = s3._Term_key
;
-- get the maximum sequence number for existing headers
CREATE TEMP TABLE maxSequence ON COMMIT DROP
AS SELECT max(sequenceNum) as maxSeq, _Object_key
FROM set3
group by _Object_key
;
-- get the maximum sequence number for any new headers
INSERT INTO maxSequence SELECT DISTINCT 0, _Object_key FROM toAdd t
WHERE not EXISTS (SELECT 1 FROM set3 s WHERE t._Object_key = s._Object_key)
;
CREATE INDEX max_idx1 on maxSequence(_Object_key)
;
-- insert new header info into VOC_AnnotHeader
FOR v_annotTypeKey, v_objectKey, v_termKey, v_isNormal, v_newSeq IN
SELECT v_annotTypeKey, t._Object_key, t._Term_key, t.isNormal, m.maxSeq
FROM toAdd t, maxSequence m 
WHERE t._Object_key = m._Object_key
LOOP 
        v_newSeq := v_newSeq + 1;
        INSERT INTO VOC_AnnotHeader values (nextval('voc_annotheader_seq'), v_annotTypeKey, v_objectKey, v_termKey, v_newSeq, v_isNormal,
                1001, 1001, NULL, NULL, current_date, current_date)
        ;
END LOOP;
-- automatically approve all annotations with one header 
CREATE TEMP TABLE toApprove ON COMMIT DROP
AS
WITH oneheader AS
(
SELECT _Object_key
FROM VOC_AnnotHeader
WHERE _AnnotType_key = 1002
AND _ApprovedBy_key is null
group by _Object_key having count(*) = 1
)
SELECT v._AnnotHeader_key
FROM oneheader o, VOC_AnnotHeader v
WHERE o._Object_key = v._Object_key
;
CREATE INDEX toApprove_idx1 ON toApprove(_AnnotHeader_key);
UPDATE VOC_AnnotHeader
SET _ApprovedBy_key = 1001, approval_date = current_date
FROM toApprove t
WHERE t._AnnotHeader_key = VOC_AnnotHeader._AnnotHeader_key
;
-- automatically set all headers to non-approved if there is at least one header (by object) that is non-approved */
CREATE TEMP TABLE toNotApprove ON COMMIT DROP
AS SELECT _AnnotHeader_key
FROM VOC_AnnotHeader v1
WHERE v1._AnnotType_key = v_annotTypeKey
AND v1._ApprovedBy_key is null
AND EXISTS (SELECT 1 FROM VOC_AnnotHeader v2 WHERE v2._AnnotType_key = v_annotTypeKey
AND v1._AnnotHeader_key != v2._AnnotHeader_key
AND v1._Object_key = v2._Object_key
AND v2._ApprovedBy_key is not null)
;
CREATE INDEX toNotApprove_idx1 ON toNotApprove(_AnnotHeader_key);
UPDATE VOC_AnnotHeader
SET _ApprovedBy_key = null, approval_date = null
FROM toNotApprove t
WHERE t._AnnotHeader_key = VOC_AnnotHeader._AnnotHeader_key
;
-- re-order
v_prevObjectKey := -1;
FOR rec IN
SELECT _AnnotHeader_key, _Object_key, sequenceNum
FROM VOC_AnnotHeader
WHERE _AnnotType_key = v_annotTypeKey
ORDER by _Object_key, sequenceNum
LOOP
    SELECT into v_pkey, v_objectKey, v_oldSeq
	rec._annotheader_key, rec._object_key, rec.sequencenum;
    IF v_objectKey != v_prevObjectKey
    THEN
	v_newSeq := 1;
    END IF;
    UPDATE VOC_AnnotHeader SET sequenceNum = v_newSeq WHERE _AnnotHeader_key = v_pkey;
    v_newSeq := v_newSeq + 1;
    v_prevObjectKey := v_objectKey;
END LOOP;
-- if VOC_AnnotHeader exists for _object_key that is not in VOC_Annot._Annottype_key = v_annottype, then delete
--DELETE from VOC_AnnotHeader h
--WHERE NOT EXISTS (SELECT 1 FROM VOC_Annot g WHERE g._AnnotType_key = v_annotTypeKey and h._object_key = g._object_key)
--;
END;
$$;


CREATE FUNCTION mgd.voc_processannotheadermissing(v_annottypekey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_objectKey int;
BEGIN
-- NAME: VOC_processAnnotHeaderMissing
-- DESCRIPTION:
--        
-- add missing VOC_AnnotHeader records by annotation type
-- INPUT:
--      
-- v_annotTypeKey : VOC_Annot._AnnotType_key
-- RETURNS:
--	VOID
--      
FOR v_objectKey IN
SELECT DISTINCT v._Object_key
FROM VOC_Annot v 
WHERE v._AnnotType_key = v_annotTypeKey
AND NOT EXISTS (select 1 FROM VOC_AnnotHeader h
	WHERE v._AnnotType_key = h._AnnotType_key 
	AND v._Object_key = h._Object_key)
LOOP
	PERFORM VOC_processAnnotHeader (1001, v_annotTypeKey, v_objectKey);
END LOOP;
END;
$$;


CREATE FUNCTION mgd.voc_resetterms(v_vocab_key integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE
v_term_key int;
v_seq int;
BEGIN
/* Re-order the sequenceNum field so that the terms are alphanumeric sorted */
v_seq := 1;
FOR v_term_key IN
SELECT _term_key
FROM VOC_Term
WHERE _Vocab_key = v_vocab_key
ORDER BY term
LOOP
    UPDATE VOC_Term SET sequenceNum = v_seq WHERE _Term_key = v_term_key;
    v_seq := v_seq + 1;
END LOOP;
RETURN;
END;
$$;


CREATE FUNCTION mgd.voc_term_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
-- NAME: VOC_Term_delete()
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
-- An example of a table that only needs ACC_Accession : MGI_Organism
-- INPUT:
--	none
-- RETURNS:
--	NEW
DELETE FROM MGI_Note m
WHERE m._Object_key = OLD._Term_key
AND m._MGIType_key = 13
;
DELETE FROM MGI_Synonym m
WHERE m._Object_key = OLD._Term_key
AND m._MGIType_key = 13
;
DELETE FROM MGI_SetMember msm 
USING MGI_Set ms
WHERE msm._Object_key = OLD._Term_key
AND msm._Set_key = ms._Set_key
AND ms._MGIType_key = 13
;
DELETE FROM DAG_Node dnode
USING DAG_DAG ddag
WHERE dnode._Object_key = OLD._Term_key
AND dnode._DAG_key = ddag._DAG_key
AND ddag._MGIType_key = 13
;
DELETE FROM ACC_Accession a
WHERE a._Object_key = OLD._Term_key
AND a._MGIType_key = 13
;
DELETE FROM VOC_Annot a
WHERE a._Object_key =  OLD._Term_key
AND a._AnnotType_key = 1024
;
DELETE FROM MGI_Relationship r 
WHERE r._Object_key_1 = OLD._Term_key 
AND r._Category_key = 1005
;
RETURN NEW;
END;
$$;



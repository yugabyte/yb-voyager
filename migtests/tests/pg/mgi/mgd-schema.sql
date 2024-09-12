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
-- Name: mgd; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA mgd;


--
-- Name: acc_accession_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.acc_accession_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: ACC_Accession_delete()
--
-- DESCRIPTOIN:
--	
-- 1) If deleting MGI Image Pixel number, then nullify X/Y Dimensions of IMG_Image record TR#134
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: acc_assignj(integer, integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.acc_assignj(v_userkey integer, v_objectkey integer, v_nextmgi integer DEFAULT '-1'::integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
j_exists int;

BEGIN

--
-- NAME: ACC_assignJ
--
-- DESCRIPTION:
--        
-- To assign a new J: to a Reference
--
-- 1:Calls ACC_assignMGI() sending prefixPart = 'J:'
--
-- INPUT:
--      
-- v_userKey   : MGI_User._User_key
-- v_objectKey : ACC_Accession._Object_key
-- v_nextMGI   : if -1, then ACC_assignMGI() will use the next available J:
--		 else, try to use the v_nextMGI as the J: (an override)
--
-- RETURNS:
--	VOID
--      
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

--
-- if this object already has a J:, return/do nothing
--
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


--
-- Name: acc_assignmgi(integer, integer, text, text, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.acc_assignmgi(v_userkey integer, v_objectkey integer, v_mgitype text, v_prefixpart text DEFAULT 'MGI:'::text, v_nextmgi integer DEFAULT '-1'::integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_nextACC int;
v_mgiTypeKey int;
v_accID acc_accession.accid%TYPE;
v_rrID acc_accession.accid%TYPE;
v_preferred int = 1;
v_private int = 0;

BEGIN

--
-- NAME: ACC_assignMGI
--
-- DESCRIPTION:
--        
-- To assign a new MGI id (MGI:xxxx) or new J (J:xxxx)
-- Add RRID for Genotypes objects (_MGIType_key = 12)
-- Increment ACC_AccessionMax.maxNumericPart
--
-- INPUT:
--      
-- v_userKey                        : MGI_User._User_key
-- v_objectKey                      : ACC_Accession._Object_key
-- v_mgiType acc_mgitype.name%TYPE  : ACC_Accession._MGIType_key (MGI: or J:)
-- v_prefixPart acc_accession.prefixpart%TYPE : ACC_Accession.prefixPart
--		The default is 'MGI:', but 'J:' is also valid
-- v_nextMGI int DEFAULT -1 : if -1 then use next available MGI id
--		this sp does not allow an override of a generated MGI accession id
-- RETURNS:
--	VOID
--      

v_nextACC := max(_Accession_key) + 1 FROM ACC_Accession;
v_mgiTypeKey := _MGIType_key FROM ACC_MGIType WHERE name = v_mgiType;

IF v_nextMGI = -1
THEN
	SELECT INTO v_nextMGI maxNumericPart + 1
	FROM ACC_AccessionMax WHERE prefixPart = v_prefixPart;
ELSIF v_prefixPart != 'J:'
THEN
    RAISE EXCEPTION E'Cannot override generation of MGI accession number';
    RETURN;
END IF;

-- check if v_nextACC already exists in ACC_Accession table
--IF (SELECT count(*) FROM ACC_Accession
--    WHERE _MGIType_key = v_mgiTypeKey
--    AND prefixPart = v_prefixPart
--    AND numericPart = v_nextACC) > 0
--THEN
--    RAISE EXCEPTION 'v_nextMGI already exists in ACC_Accession: %', v_nextMGI;
--    RETURN;
--END IF;

v_accID := v_prefixPart || v_nextMGI::char(30);

INSERT INTO ACC_Accession 
(_Accession_key, accID, prefixPart, numericPart, 
_LogicalDB_key, _Object_key, _MGIType_key, preferred, private, 
_CreatedBy_key, _ModifiedBy_key)
VALUES(v_nextACC, v_accID, v_prefixPart, v_nextMGI, 1, v_objectKey, 
v_mgiTypeKey, v_preferred, v_private, v_userKey, v_userKey);

IF (SELECT maxNumericPart FROM ACC_AccessionMax
    WHERE prefixPart = v_prefixPart) <= v_nextMGI
THEN
	UPDATE ACC_AccessionMax 
	SET maxNumericPart = v_nextMGI 
	WHERE prefixPart = v_prefixPart;
END IF;

--
-- add RRID for Genotypes
--
IF v_mgiTypeKey = 12
THEN
    v_nextACC := max(_Accession_key) + 1 FROM ACC_Accession;
    v_rrID := 'RRID:' || v_accID::char(30); 
    INSERT INTO ACC_Accession
	(_Accession_key, accID, prefixPart, numericPart,
	_LogicalDB_key, _Object_key, _MGIType_key, preferred, private,
	_CreatedBy_key, _ModifiedBy_key)
	VALUES(v_nextACC, v_rrID, v_prefixPart, v_nextMGI, 179, v_objectKey,
	v_mgiTypeKey, v_preferred, v_private, v_userKey, v_userKey);
END IF;

RETURN;

END;
$$;


--
-- Name: acc_delete_byacckey(integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.acc_delete_byacckey(v_acckey integer, v_refskey integer DEFAULT '-1'::integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

BEGIN

--
-- NAME: ACC_delete_byAccKey
--
-- DESCRIPTION:
--        
-- To delete ACC_Accession and ACC_AccessionReference
-- This may be obsolete now that referential integrity exists
--
-- INPUT:
--      
-- v_accKey  : ACC_Accession._Accession_key
-- v_refsKey : BIB_Refs._Refs_key
--
-- RETURNS:
--	VOID
--      
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


--
-- Name: acc_insert(integer, integer, text, integer, text, integer, integer, integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.acc_insert(v_userkey integer, v_objectkey integer, v_accid text, v_logicaldb integer, v_mgitype text, v_refskey integer DEFAULT '-1'::integer, v_preferred integer DEFAULT 1, v_private integer DEFAULT 0, v_dosplit integer DEFAULT 1) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_nextACC int;
v_mgiTypeKey int;
v_prefixPart acc_accession.prefixPart%TYPE;
v_numericPart acc_accession.prefixPart%Type;

BEGIN

--
-- NAME: ACC_insert
--
-- DESCRIPTION:
--        
-- To add new ACC_Accession record with out checking Accession id rules
-- If reference is given, insert record into ACC_AccessionReference
--
-- INPUT:
--      
-- v_userKey                        : MGI_User._User_key
-- v_objectKey                      : ACC_Accession._Object_key
-- v_accID acc_accession.accid%TYPE : ACC_Accession.accID
-- v_logicalDB                      : ACC_Accession._LogicalDB_key
-- v_mgiType acc_mgitype.name%TYPE  : ACC_Accession._MGIType_key
-- v_refsKey int DEFAULT -1         : BIB_Refs._Refs_key; if Reference, then call ACCRef_insert()
-- v_preferred int DEFAULT 1        : ACC_Accession.prefixPart
-- v_private int DEFAULT 0          : ACC_Accession.private
-- v_dosplit int DEFAULT 1          : if 1, split the accession id into prefixpart/numericpart
--
-- RETURNS:
--      
--

IF v_accID IS NULL
THEN
	RETURN;
END IF;

v_nextACC := max(_Accession_key) + 1 FROM ACC_Accession;

v_mgiTypeKey := _MGIType_key FROM ACC_MGIType WHERE name = v_mgiType;
v_prefixPart := v_accID;
v_numericPart := '';

-- skip the splitting...for example, the Reference/DOI ids are not split

IF v_dosplit
THEN
	-- split accession id INTO prefixPart/numericPart
	SELECT * FROM ACC_split(v_accID) INTO v_prefixPart, v_numericPart;
END IF;

IF (select count(accid) from acc_accession 
        where _mgitype_key = 10 and _logicaldb_key = v_logicalDB and accid = v_accID
        group by _logicaldb_key, accid having count(*) >= 1)
THEN
        RAISE EXCEPTION E'Cannot assign same Registry:Accession Id to > 1 Strain';
        RETURN;
END IF;

IF v_numericPart = ''
THEN
	INSERT INTO ACC_Accession
	(_Accession_key, accID, prefixPart, numericPart, _LogicalDB_key, _Object_key, 
 	 _MGIType_key, private, preferred, _CreatedBy_key, _ModifiedBy_key)
	VALUES(v_nextACC, v_accID, v_prefixPart, null, 
               v_logicalDB, v_objectKey, v_mgiTypeKey, v_private, v_preferred, v_userKey, v_userKey);
ELSE
	INSERT INTO ACC_Accession
	(_Accession_key, accID, prefixPart, numericPart, _LogicalDB_key, _Object_key, 
 	 _MGIType_key, private, preferred, _CreatedBy_key, _ModifiedBy_key)
	VALUES(v_nextACC, v_accID, v_prefixPart, v_numericPart::integer, 
               v_logicalDB, v_objectKey, v_mgiTypeKey, v_private, v_preferred, v_userKey, v_userKey);
END IF;

IF v_refsKey != -1
THEN
	PERFORM ACCRef_insert(v_userKey, v_nextAcc, v_refsKey);
END IF;

RETURN;

END;
$$;


--
-- Name: acc_setmax(integer, text); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.acc_setmax(v_increment integer, v_prefixpart text DEFAULT 'MGI:'::text) RETURNS void
    LANGUAGE plpgsql
    AS $$

BEGIN

--
-- NAME: ACC_setMax
--
-- DESCRIPTION:
--        
-- To update/set the ACC_AccessionMax.maxNumericPart for given type
--
-- INPUT:
--      
-- v_increment : the amount to which to increment the maxNumericPart
-- v_prefixPart acc_accession.prefixPart%TYPE DEFAULT 'MGI:'
--	the default is 'MGI:' type
--      else enter a valid ACC_Accession.prefixPart ('J:')
--
-- RETURNS:
--	VOID
--      

UPDATE ACC_AccessionMax
SET maxNumericPart = maxNumericPart + v_increment
WHERE prefixPart = v_prefixPart
;

END;
$$;


--
-- Name: acc_split(text); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.acc_split(v_accid text, OUT v_prefixpart text, OUT v_numericpart text) RETURNS record
    LANGUAGE plpgsql
    AS $$

BEGIN

--
-- NAME: ACC_split
--
-- DESCRIPTION:
--        
-- To split an Accession ID into a prefixPart and a numericPart
--
-- to call : select v_prefixPart, v_numericPart from ACC_split('MGI:12345');
-- example : A2ALT2 -> (A2ALT, 2)
-- example : MGI:12345 -> (MGI:, 12345)
--
-- INPUT:
--      
-- v_accID acc_accession.accid%TYPE                : ACC_Accession.accID
--
-- RETURNS:
--	prefixPart  : ACC_Accession.prefixPart%TYPE
--      numericPart : ACC_Accession.numericPart%TYPE
--      
--

-- v_prefixPart is alphanumeric
v_prefixPart := (SELECT (regexp_matches(v_accID, E'^((.*[^0-9])?)([0-9]*)', 'g'))[2]);

-- v_numericPart is numeric only
v_numericPart := (SELECT (regexp_matches(v_accID, E'^((.*[^0-9])?)([0-9]*)', 'g'))[3]);

RETURN;

END;
$$;


--
-- Name: acc_update(integer, integer, text, integer, integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.acc_update(v_userkey integer, v_acckey integer, v_accid text, v_private integer DEFAULT 0, v_origrefskey integer DEFAULT '-1'::integer, v_refskey integer DEFAULT '-1'::integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_prefixPart acc_accession.prefixPart%TYPE;
v_numericPart acc_accession.prefixPart%TYPE;

BEGIN

--
-- NAME: ACC_update
--
-- DESCRIPTION:
--        
-- To update an ACC_Accession, ACC_AccessionReference records
--
-- INPUT:
--      
-- v_userKey     : MGI_User._User_key
-- v_accKey      : ACC_Accession._Accession_key
-- v_accID       : ACC_Accession.accID
-- v_origRefsKey : original ACC_AccessionReference._Refs_key
-- v_refsKey     : new ACC_AccessionReference._Refs_key
-- v_private     : ACC_Accessioin.private
--
-- RETURNS:
--	VOID
--      

v_numericPart := '';

IF v_accID IS NULL
THEN
	select ACC_delete_byAccKey (v_accKey);
ELSE
        -- split accession id INTO prefixPart/numericPart

        SELECT * FROM ACC_split(v_accID) INTO v_prefixPart, v_numericPart;

	IF (v_prefixPart = 'J:' or substring(v_prefixPart,1,4) = 'MGD-')
	THEN
		IF (select count(*) from ACC_Accession
		    where numericPart = v_numericPart::integer
			  and prefixPart = v_prefixPart) >= 1
		THEN
                        RAISE EXCEPTION E'Duplicate MGI Accession Number';
                        RETURN;
		END IF;

	END IF;

	IF v_numericPart = ''
	THEN
		update ACC_Accession
	       	set accID = v_accID,
		   	prefixPart = v_prefixPart,
		   	numericPart = null,
                        private = v_private,
		   	_ModifiedBy_key = v_userKey,
		   	modification_date = now()
	       	where _Accession_key = v_accKey;
        ELSE
		update ACC_Accession
	       	set accID = v_accID,
		   	prefixPart = v_prefixPart,
		   	numericPart = v_numericPart::integer,
                        private = v_private,
		   	_ModifiedBy_key = v_userKey,
		   	modification_date = now()
	       	where _Accession_key = v_accKey;
	END IF;

	IF v_refsKey > 0
	THEN
		update ACC_AccessionReference
		       set _Refs_key = v_refsKey,
		           _ModifiedBy_key = v_userKey,
		           modification_date = now()
		           where _Accession_key = v_accKey and _Refs_key = v_origRefsKey;
	END IF;

END IF;

END;
$$;


--
-- Name: accref_insert(integer, integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.accref_insert(v_userkey integer, v_acckey integer, v_refskey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

BEGIN

--
-- NAME: ACCRef_insert
--
-- DESCRIPTION:
--    
-- To insert a new reference record into ACC_AccessionReference
--
-- INPUT:
--      
-- v_userKey : MGI_User._User_key
-- v_accKey  : ACC_Accession._Accession_key
-- v_refsKey : BIB_Refs._Refs_key
--
-- RETURNS:
--	VOID
--      
--

-- Insert record into ACC_AccessionReference table

INSERT INTO ACC_AccessionReference
(_Accession_key, _Refs_key, _CreatedBy_key, _ModifiedBy_key)
VALUES(v_accKey, v_refsKey, v_userKey, v_userKey)
;

END;
$$;


--
-- Name: accref_process(integer, integer, integer, text, integer, text, integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.accref_process(v_userkey integer, v_objectkey integer, v_refskey integer, v_accid text, v_logicaldb integer, v_mgitype text, v_preferred integer DEFAULT 1, v_private integer DEFAULT 0) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_accKey int;

BEGIN

--
-- NAME: ACCRef_process
--
-- DESCRIPTION:
--        
-- To add a new Accession/Reference row to ACC_AccessionReference
--
-- If an Accession object already exists, then call ACCRef_insert() to add the new Accession/Reference row
-- Else, call ACC_insert() to add a new Accession object and a new Accession/Reference row
--
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
--
-- RETURNS:
--	VOID
--      
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


--
-- Name: all_allele_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.all_allele_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: ALL_Allele_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

--
-- NOTE:  Any changes should also be reflected in:
--     pgdbutilities/sp/MGI_deletePrivateData.csh
--

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


--
-- Name: all_allele_insert(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.all_allele_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$

BEGIN

--
-- NAME: ALL_Allele_insert()
--
-- DESCRIPTOIN:
--
--	this insert trigger will call ACC_assignMGI
--	in order to add a distinct MGI accession id
--	to the NEW object
--
-- RULES:
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

PERFORM ACC_assignMGI(1001, NEW._Allele_key, 'Allele');

RETURN NEW;

END;
$$;


--
-- Name: all_allele_update(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.all_allele_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$

BEGIN

--
-- NAME: ALL_Allele_update()
-- NAME: ALL_Allele_update2()
--
-- DESCRIPTOIN:
--
-- 	1) GO check: approved/autoload may exist in VOC_Evidence/inferredFrom
--	2) if setting Allele Status = Deleted, then check cross-references
--
-- RULES:
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

--
-- in progress =  847111
-- reserved = 847113
-- approved = 847114
-- autoload = 3983021
-- deleted =  847112
--
-- GO check: approved/autoload
--

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

--
-- if setting Allele Status = Deleted, then check cross-references
--

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


--
-- Name: all_cellline_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.all_cellline_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: ALL_CellLine_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: all_cellline_update1(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.all_cellline_update1() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: ALL_CellLine_update1()
-- NAME: ALL_CellLine_update2()
--
-- DESCRIPTOIN:
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

-- update strains of mutant cell lines derived from this parent line

UPDATE ALL_CellLine
SET _Strain_key = NEW._Strain_key
FROM ALL_CellLine_Derivation d
WHERE NEW._CellLine_key = d._ParentCellLine_key
AND d._Derivation_key = ALL_CellLine._Derivation_key
;

--
-- Previously, when Strain was updated, we also needed to update all
-- Alleles which reference this ES Cell Line AND the old Strain (excluding
-- NA, NS, Other (see notes) cell lines).  Now, that is done by a nightly
-- script rather than in this trigger.
--

RETURN NEW;

END;
$$;


--
-- Name: all_cellline_update2(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.all_cellline_update2() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- If a mutant cell line has a change in its derivation key, then its strain
-- key must change to match the strain of its new parent cell line.
--

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


--
-- Name: all_convertallele(integer, integer, text, text, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.all_convertallele(v_userkey integer, v_markerkey integer, v_oldsymbol text, v_newsymbol text, v_alleleof integer DEFAULT 0) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_alleleKey int;

BEGIN

--
-- NAME: ALL_convertAllele
--
-- DESCRIPTION:
--        
-- Convert allele symbols of v_markerKey using v_oldSymbol and v_newSymbol values;
-- do NOT call this with a null value of v_markerKey, or it will fail and roll
-- back your transaction.
--
-- INPUT:
--      
-- v_userKey   : MGI_User._User_key
-- v_markerKey : MRK_Marker._Marker_key
-- v_oldSymbol mrk_marker.symbol%TYPE : old marker symbol
-- v_newSymbol mrk_marker.symbol%TYPE : new marker symbol
-- v_alleleOf int DEFAULT 0 : is old Symbol an Allele of the new Symbol?
--
-- RETURNS:
--	VOID
--      

--
-- If the marker key is null, then this procedure will not work.  Bail out
-- now before anything gets mixed up.
--

IF v_markerKey IS NULL
THEN
	RAISE EXCEPTION E'ALL_convertAllele : Cannot update symbols for allele which have no marker.';
	RETURN;
END IF;

--
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


--
-- Name: all_createwildtype(integer, integer, text); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.all_createwildtype(v_userkey integer, v_markerkey integer, v_symbol text) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_asymbol all_allele.symbol%TYPE;

BEGIN

--
-- NAME: ALL_createWildType
--
-- DESCRIPTION:
--        
-- Create a Wild Type Allele
-- Use Reference = J:23000
-- Set all other attributes = Not Applicable
--
-- INPUT:
--
-- v_userKey   : MGI_User._User_key
-- v_markerKey : MRK_Marker._Marker_key
-- v_symbol mrk_marker.symbol%TYPE
--
-- RETURNS:
--	VOID
--      

v_asymbol := v_symbol || '<+>';

PERFORM ALL_insertAllele (
	v_userKey,
	v_markerKey,
	v_asymbol,
	'wild type',
	null,
	1,
	v_userKey,
	v_userKey,
	v_userKey,
	current_date,
	-2,
	'Not Applicable',
	'Not Applicable',
        'Approved',
        null,
        'Not Applicable',
        'Not Specified',
        'Not Specified',
        'Curated'
	);

IF NOT FOUND
THEN
	RAISE EXCEPTION E'ALL_createWildType: PERFORM ALL_insertAllele failed';
END IF;

END;
$$;


--
-- Name: all_insertallele(integer, integer, text, text, integer, integer, integer, integer, integer, timestamp without time zone, integer, text, text, text, text, text, text, text, text); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.all_insertallele(v_userkey integer, v_markerkey integer, v_symbol text, v_name text, v_refskey integer DEFAULT NULL::integer, v_iswildtype integer DEFAULT 0, v_createdby integer DEFAULT 1001, v_modifiedby integer DEFAULT 1001, v_approvedby integer DEFAULT NULL::integer, v_approval_date timestamp without time zone DEFAULT NULL::timestamp without time zone, v_strainkey integer DEFAULT '-1'::integer, v_amode text DEFAULT 'Not Specified'::text, v_atype text DEFAULT 'Not Specified'::text, v_astatus text DEFAULT 'Approved'::text, v_oldsymbol text DEFAULT NULL::text, v_transmission text DEFAULT 'Not Applicable'::text, v_collection text DEFAULT 'Not Specified'::text, v_qualifier text DEFAULT 'Not Specified'::text, v_mrkstatus text DEFAULT 'Curated'::text) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_alleleKey int;

v_createdByKey int;
v_modifiedByKey int;
v_approvedByKey int;

v_modeKey int;
v_typeKey int;
v_statusKey int;
v_transmissionKey int;
v_collectionKey int;
v_assocKey int;
v_qualifierKey int;
v_mrkstatusKey int;

v_isExtinct all_allele.isextinct%TYPE;
v_isMixed all_allele.ismixed%TYPE;

BEGIN

--
-- NAME: ALL_insertAllele
--
-- DESCRIPTION:
--        
-- To insert a new Allele into ALL_Allele
-- Also calls: MGI_insertReferenceAssoc() to create 'Original'/1011 reference
--
-- INPUT:
--      
-- see below
--
-- RETURNS:
--	VOID
--      

v_alleleKey := nextval('all_allele_seq');

IF v_createdBy IS NULL
THEN
	v_createdByKey := v_userKey;
ELSE
	v_createdByKey := _User_key from MGI_User where login = current_user;
END IF;

IF v_modifiedBy IS NULL
THEN
	v_modifiedByKey := v_userKey;
ELSE
	v_modifiedByKey := _User_key from MGI_User where login = current_user;
END IF;

IF v_approvedBy IS NULL
THEN
	v_approvedByKey := v_userKey;
ELSE
	v_approvedByKey := _User_key from MGI_User where login = current_user;
END IF;

v_modeKey := _Term_key from VOC_Term where _Vocab_key = 35 AND term = v_amode;
v_typeKey := _Term_key from VOC_Term where _Vocab_key = 38 AND term = v_atype;
v_statusKey := _Term_key from VOC_Term where _Vocab_key = 37 AND term = v_astatus;
v_transmissionKey := _Term_key from VOC_Term where _Vocab_key = 61 AND term = v_transmission;
v_collectionKey := _Term_key from VOC_Term where _Vocab_key = 92 AND term = v_collection;
v_qualifierKey := _Term_key from VOC_Term where _Vocab_key = 70 AND term = v_qualifier;
v_mrkstatusKey := _Term_key from VOC_Term where _Vocab_key = 73 AND term = v_mrkstatus;
v_isExtinct := 0;
v_isMixed := 0;

IF v_astatus = 'Approved' AND v_approval_date IS NULL
THEN
	v_approval_date := now();
END IF;

/* Insert New Allele into ALL_Allele */

INSERT INTO ALL_Allele
(_Allele_key, _Marker_key, _Strain_key, _Mode_key, _Allele_Type_key, _Allele_Status_key, _Transmission_key,
        _Collection_key, symbol, name, isWildType, isExtinct, isMixed,
	_Refs_key, _MarkerAllele_Status_key,
        _CreatedBy_key, _ModifiedBy_key, _ApprovedBy_key, approval_date, creation_date, modification_date)
VALUES(v_alleleKey, v_markerKey, v_strainKey, v_modeKey, v_typeKey, v_statusKey, v_transmissionKey,
        v_collectionKey, v_symbol, v_name, v_isWildType, v_isExtinct, v_isMixed,
	v_refsKey, v_mrkstatusKey,
        v_createdByKey, v_modifiedByKey, v_approvedByKey, v_approval_date, now(), now())
;

IF v_refsKey IS NOT NULL
THEN
        PERFORM MGI_insertReferenceAssoc (v_userKey, 11, v_alleleKey, v_refsKey, 1011);
END IF;

--IF (v_oldSymbol IS NOT NULL) AND (v_markerKey IS NOT NULL)
--THEN
        --UPDATE MLD_Expt_Marker SET _Allele_key = v_alleleKey 
	--WHERE _Marker_key = v_markerKey AND gene = v_oldSymbol;
--END IF;

END;

$$;


--
-- Name: all_mergeallele(integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.all_mergeallele(v_oldallelekey integer, v_newallelekey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

BEGIN

--
-- NAME: ALL_mergeAllele
--
-- DESCRIPTION:
--        
-- To re-set _Allele_key from old _Allele_key to new _Allele_key
-- called from ALL_mergeWildTypes()
--
-- INPUT:
--      
-- v_oldAlleleKey : ALL_Allele._Allele_key (old)
-- v_newAlleleKey : ALL_Allele._Allele_key (new)
--
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


--
-- Name: all_mergewildtypes(integer, integer, text, text); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.all_mergewildtypes(v_oldkey integer, v_newkey integer, v_oldsymbol text, v_newsymbol text) RETURNS void
    LANGUAGE plpgsql
    AS $$

--
-- do NOT call this procedure with one or both of the marker keys being null,
-- or it will fail and will roll back your transaction
--

DECLARE

v_oldAlleleKey int;
v_newAlleleKey int;

BEGIN

--
-- NAME: ALL_mergeWildTypes
--
-- DESCRIPTION:
--        
-- To merge wild type alleles : calls ALL_mergeAllele()
-- exception if oldSymbol or newSymbol is null
--
-- INPUT:
--
-- v_oldKey : ALL_Allele._Allele_key
-- v_newKey : ALL_Allele._Allele_key
-- v_oldSymbol mrk_marker.symbol%TYPE
-- v_newSymbol mrk_marker.symbol%TYPE
--      
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


--
-- Name: all_reloadlabel(integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.all_reloadlabel(v_allelekey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_userKey int;
v_labelstatusKey int;
v_priority int;
v_label all_label.label%TYPE;
v_labelType all_label.labelType%TYPE;
v_labelTypeName all_label.labelTypeName%TYPE;

BEGIN

--
-- NAME: ALL_reloadLabel
--
-- DESCRIPTION:
--        
-- reload ALL_Label for given Allele
--
-- INPUT:
--      
-- v_alleleKey : ALL_Allele._Allele_key
--
-- RETURNS:
--	VOID
--      

-- Delete all ALL_Label records for a Allele and regenerate

DELETE FROM ALL_Label WHERE _Allele_key = v_alleleKey;

FOR v_labelstatusKey, v_priority, v_label, v_labelType, v_labelTypeName IN
SELECT DISTINCT 1 as _Label_Status_key, 1 as priority, 
a.symbol, 'AS' as labelType, 'allele symbol' as labelTypeName
FROM ALL_Allele a
WHERE a._Allele_key = v_alleleKey
AND a.isWildType = 0
UNION 
SELECT distinct 1 as _Label_Status_key, 2 as priority,
a.name, 'AN' as labelType, 'allele name' as labelTypeName
FROM ALL_Allele a
WHERE a._Allele_key = v_alleleKey
AND a.isWildType = 0
UNION
SELECT 1 as _Label_Status_key, 3 as priority,
s.synonym, 'AY' as labelType, 'synonym' as labelTypeName
FROM MGI_Synonym_Allele_View s
WHERE s._Object_key = v_alleleKey
LOOP
	INSERT INTO ALL_Label 
	(_Allele_key, _Label_Status_key, priority, label, labelType, labelTypeName, creation_date, modification_date)
	VALUES (v_alleleKey, v_labelstatusKey, v_priority, v_label, v_labelType, v_labelTypeName, now(), now())
	;
END LOOP;

END;
$$;


--
-- Name: all_variant_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.all_variant_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: ALL_Variant_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

--
-- NOTE:  Any changes should also be reflected in:
--     pgdbutilities/sp/MGI_deletePrivateData.csh
--

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


--
-- Name: bib_keepwfrelevance(integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.bib_keepwfrelevance(v_refskey integer, v_userkey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
 
BEGIN

--
-- NAME: BIB_keepWFRelevance
--
-- DESCRIPTION:
--        
-- To set the current Workflow Relevance = keep, if it does not already exist
--
-- INPUT:
--	None
--
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


--
-- Name: bib_refs_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.bib_refs_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: BIB_Refs_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: bib_refs_insert(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.bib_refs_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$

DECLARE
rec record;

BEGIN

--
-- NAME: BIB_Refs_insert()
--
-- DESCRIPTOIN:
--
--	this insert trigger will call ACC_assignMGI
--	in order to add a distinct MGI accession id
--	to the NEW object
--
--	adds: J:
--	adds: BIB_Workflow_Status, one row per Group, status = 'New'
--	adds: BIB_Workflow_Data, supplimental term = 'No supplemental data' (34026998)
--	adds: BIB_Workflow_Data, extracted text section = 'body' (48804490)
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: bib_reloadcache(integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.bib_reloadcache(v_refskey integer DEFAULT '-1'::integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

BEGIN

--
-- NAME: BIB_reloadCache
--
-- DESCRIPTION:
--        
-- To delete/reload the BIB_Citation_Cache
--
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
--

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


--
-- Name: bib_updatewfstatusap(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.bib_updatewfstatusap() RETURNS void
    LANGUAGE plpgsql
    AS $$
 
DECLARE
rec record;

BEGIN

--
-- NAME: BIB_updateWFStatusAP
--
-- DESCRIPTION:
--        
-- To set the group AP/current Workflow Status = Full-coded
--
-- where the Reference exists in AP Annotation (_AnnotType_key = 1002)
-- and the group AP/current Workflow Status is *not* Full-coded
-- 
-- To set the group AP/current Workflow Status = Indexed
--
-- where the Reference exists for an Allele in MGI_Reference_Assoc but there
-- is no MP Annotation snd the group AP/current Workflow Status is *not*
-- Indexed
--
-- if either: 
-- To set the current Workflow Relevance = keep
--
-- INPUT:
--	None
--
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


--
-- Name: bib_updatewfstatusgo(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.bib_updatewfstatusgo() RETURNS void
    LANGUAGE plpgsql
    AS $$
 
DECLARE
rec record;

BEGIN

--
-- NAME: BIB_updateWFStatusGO
--
-- DESCRIPTION:
--        
-- To set the group GO/current Workflow Status = Full-coded
--
-- where the Reference exists in GO Annotation (_AnnotType_key = 1000)
-- and the group GO/current Workflow Status is *not* Full-coded
--
-- INPUT:
--	None
--
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


--
-- Name: bib_updatewfstatusgxd(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.bib_updatewfstatusgxd() RETURNS void
    LANGUAGE plpgsql
    AS $$
 
DECLARE
rec record;

BEGIN

--
-- NAME: BIB_updateWFStatusGXD
--
-- DESCRIPTION:
--        
-- To set the group GXD/current Workflow Status = Full-coded
--
-- where the Reference exists in GXD_Assay
-- and the group GXD/current Workflow Status is *not* Full-coded
--
-- INPUT:
--	None
--
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


--
-- Name: bib_updatewfstatusqtl(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.bib_updatewfstatusqtl() RETURNS void
    LANGUAGE plpgsql
    AS $$
 
DECLARE
rec record;

BEGIN

--
-- NAME: BIB_updateWFStatusQTL
--
-- DESCRIPTION:
--        
-- To set the group QTL/current Workflow Status = Full-coded
--
-- where the Reference exists in QTL Mapping Experiment
-- and the group QTL/current Workflow Status is *not* Full-coded 
--
-- INPUT:
--	None
--
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


--
-- Name: gxd_addcelltypeset(text, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_addcelltypeset(v_createdby text, v_assaykey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_newSetMemberKey int;
v_newSeqKey int;
v_userKey int;

BEGIN

--
-- NAME: GXD_addCellTypeSet
--
-- DESCRIPTION:
--
-- To add Assay/Cell Types to Cell Ontology set for the clipboard
-- (MGI_SetMember)
--        
-- INPUT:
--      
-- v_createdBy : MGI_User.name
-- v_assayKey : GXD_Assay._Assay_key
--
-- RETURNS:
--	VOID
--

--
-- find existing Cell Ontology terms by Assay
-- add them to MGI_SetMember (clipboard)
--
-- exclude existing Cell Ontology terms that already exist in the clipboard
-- that is, do not add dupliate structures to the clipboard
--

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


--
-- Name: gxd_addemapaset(text, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_addemapaset(v_createdby text, v_assaykey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_newSetMemberKey int;
v_newSetMemberEmapaKey int;
v_newSeqKey int;
v_userKey int;

BEGIN

--
-- NAME: GXD_addEMAPASet
--
-- DESCRIPTION:
--
-- To add Assay/EMAPA/Stage to EMAPA set for the clipboard
-- (MGI_SetMember, MGI_SetMember_EMAPA)
--        
-- INPUT:
--      
-- v_createdBy : MGI_User.name
-- v_assayKey : GXD_Assay._Assay_key
--
-- RETURNS:
--	VOID
--

--
-- find existing EMAPA/Stage terms by Assay
-- add them to MGI_SetMember/MGI_SetMember_EMAPA (clipboard)
--
-- exclude existing EMAPA/Stages terms that already exist in the clipboard
-- that is, do not add dupliate structures to the clipboard
--

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


--
-- Name: gxd_addgenotypeset(text, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_addgenotypeset(v_createdby text, v_assaykey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_newSetMemberKey int;
v_newSeqKey int;
v_userKey int;

BEGIN

--
-- NAME: GXD_addGenotypeSet
--
-- DESCRIPTION:
--
-- To add Assay/Genotypes to Genotype set for the clipboard
-- (MGI_SetMember)
--        
-- INPUT:
--      
-- v_createdBy : MGI_User.name
-- v_assayKey : GXD_Assay._Assay_key
--
-- RETURNS:
--	VOID
--

--
-- find existing Genotype by Assay
-- add them to MGI_SetMember (clipboard)
--
-- exclude existing Genotype that already exist in the clipboard
-- that is, do not add dupliate structures to the clipboard
--

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


--
-- Name: gxd_allelepair_insert(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_allelepair_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: GXD_AllelePair_insert()
--
-- DESCRIPTOIN:
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: gxd_antibody_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_antibody_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: GXD_Antibody_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: gxd_antibody_insert(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_antibody_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: GXD_Antibody_insert()
--
-- DESCRIPTOIN:
--
--	this insert trigger will call ACC_assignMGI
--	in order to add a distinct MGI accession id
--	to the NEW object
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

PERFORM ACC_assignMGI(1001, NEW._Antibody_key, 'Antibody');

RETURN NEW;

END;
$$;


--
-- Name: gxd_antigen_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_antigen_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: GXD_Antigen_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: gxd_antigen_insert(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_antigen_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: GXD_Antigen_insert()
--
-- DESCRIPTOIN:
--
--	this insert trigger will call ACC_assignMGI
--	in order to add a distinct MGI accession id
--	to the NEW object
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

PERFORM ACC_assignMGI(1001, NEW._Antigen_key, 'Antigen');

RETURN NEW;

END;
$$;


--
-- Name: gxd_assay_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_assay_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: GXD_Assay_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: gxd_assay_insert(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_assay_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$

DECLARE
v_assocKey int;

BEGIN

--
-- NAME: GXD_Assay_insert()
--
-- DESCRIPTOIN:
--
--	this insert trigger will call ACC_assignMGI
--	in order to add a distinct MGI accession id
--	to the NEW object
--
--	1) if the Reference exists as group = 'Expression'
--		in any BIB_Workflow_Status,
--              and BIB_Workflow_Status is not equal to "Full-coded"
--              and BIB_Workflow_Status.isCurrent = 1
--	   then
--		set existing BIB_Workflow_Status.isCurrent = 0
--		add new BIB_Workflow_Status._Status_key = 'Full-coded'
--
-- changes to this trigger require changes to procedure/BIB_updateWFStatusQTL_create.object
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: gxd_checkduplicategenotype(bigint); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_checkduplicategenotype(v_genotypekey bigint) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_dupgenotypeKey int;
v_isDuplicate int;

BEGIN

--
-- NAME: GXD_checkDuplicateGenotype
--
-- DESCRIPTION:
--        
-- To check if Genotype is a duplicate
--
-- INPUT:
--      
-- v_genotypeKey : GXD_Genotype._Genotype_key
--
-- RETURNS:
--	always VOID
--	but RAISE EXCPEPTION if duplicate Genotype is found
--      

--
-- Check if the given genotype record is a duplicate.
-- If it is a duplicate, the transaction is rolled back. 
--
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


--
-- Name: gxd_gelrow_insert(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_gelrow_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: GXD_GelRow_insert()
--
-- DESCRIPTOIN:
--	
-- 1) If Gel Row Size is entered, Gel Row Units must be specified.
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

IF NEW.size IS NOT NULL AND NEW._GelUnits_key < 0
THEN
  RAISE EXCEPTION E'If Gel Row Size is entered, Gel Row Units must be specified.';
END IF;

RETURN NEW;

END;
$$;


--
-- Name: gxd_genotype_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_genotype_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: GXD_Genotype_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: gxd_genotype_insert(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_genotype_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: GXD_Genotype_insert()
--
-- DESCRIPTOIN:
--
--	this insert trigger will call ACC_assignMGI
--	in order to add a distinct MGI accession id
--	to the NEW object
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

PERFORM ACC_assignMGI(1001, NEW._Genotype_key, 'Genotype');

RETURN NEW;

END;
$$;


--
-- Name: gxd_getgenotypesdatasets(integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_getgenotypesdatasets(v_genotypekey integer) RETURNS TABLE(_refs_key integer, jnum integer, jnumid text, short_citation text, dataset text)
    LANGUAGE plpgsql
    AS $$

BEGIN

--
-- NAME: GXD_getGenotypesDataSets
--
-- DESCRIPTION:
--        
-- To select all references/data sets that a given Genotype is associated with
--
-- INPUT:
--
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


--
-- Name: gxd_getgenotypesdatasetscount(integer); Type: FUNCTION; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_htexperiment_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_htexperiment_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: GXD_HTExperiment_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: gxd_htrawsample_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_htrawsample_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: GXD_HTRawSample_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--


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


--
-- Name: gxd_htsample_ageminmax(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_htsample_ageminmax() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: GXD_HTSample_ageminmax()
--
-- DESCRIPTOIN:
--	
-- After insert or update of GXD_HTSample, set the ageMin/ageMax
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

IF (TG_OP = 'UPDATE') THEN
	PERFORM MGI_resetAgeMinMax ('GXD_HTSample', OLD._Sample_key);
ELSE
	PERFORM MGI_resetAgeMinMax ('GXD_HTSample', NEW._Sample_key);
END IF;

RETURN NEW;

END;
$$;


--
-- Name: gxd_index_insert(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_index_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$

DECLARE
v_assocKey int;

BEGIN

--
-- NAME: GXD_Index_insert()
--
-- DESCRIPTOIN:
--
--	1) if the Reference exists as group = 'Expression'
--		and status not in ('Full-coded')
--		in BIB_Workflow_Status,
--	   then
--		set existing BIB_Workflow_Status.isCurrent = 0
--		add new BIB_Workflow_Status._Status_key = 'Indexed'
--
-- changes to this trigger require changes to procedure/BIB_updateWFStatusGXD_create.object
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: gxd_index_insert_before(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_index_insert_before() RETURNS trigger
    LANGUAGE plpgsql
    AS $$

BEGIN

--
-- NAME: GXD_Index_insert_before()
--
-- DESCRIPTOIN:
--
--	set the _Priority_key/_ConditionalMutants_key values as necessary
--	BEFORE insert
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: gxd_index_update(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_index_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$

BEGIN

--
-- NAME: GXD_Index_update()
--
-- DESCRIPTOIN:
--
--	update all _Priority_key/_ConditionalMutants_key for all instances of NEW._Refs_key
--	only if index count < 2000
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: gxd_orderallelepairs(integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_orderallelepairs(v_genotypekey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_pkey int;		-- primary key of records to update
v_oldSeq int;		-- current sequence number
v_newSeq int := 1;	-- new sequence number

BEGIN

--
-- NAME: GXD_orderAllelePairs
--
-- DESCRIPTION:
--        
-- Re-order the Allele Pairs of the given Genotype
-- alphabetically by Marker Symbol
--
-- if the Genotype contains an Allele whose Compound value != Not Applicable
-- then do not reorder
--
-- INPUT:
--      
-- v_genotypeKey : GXD_Genotype._Genotype_key
--
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


--
-- Name: gxd_ordergenotypes(integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_ordergenotypes(v_allelekey integer, v_userkey integer DEFAULT 1001) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_pkey int;	-- primary key of records to UPDATE 
v_oldSeq int;	-- current sequence number
v_newSeq int;	-- new sequence number

BEGIN

--
-- NAME: GXD_orderGenotypes
--
-- DESCRIPTION:
--        
-- Load the GXD_AlleleGenotype (cache) table for the given Allele.
-- Executed after any modification to GXD_AllelePair.
-- test: 263, 8734
--
-- INPUT:
--
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


--
-- Name: gxd_ordergenotypesall(integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_ordergenotypesall(v_genotypekey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_alleleKey int;

BEGIN

--
-- NAME: GXD_orderGenotypesAll
--
-- DESCRIPTION:
--        
-- Refrest the GXD_AlleleGenotype (cache) table for all Genotypes
--
-- INPUT:
--      
-- v_genotypeKey : GXD_Genotype._Genotype_key
--
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


--
-- Name: gxd_ordergenotypesmissing(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_ordergenotypesmissing() RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_alleleKey int;

BEGIN

--
-- NAME: GXD_orderGenotypesMissing
--
-- DESCRIPTION:
--        
-- Refresh the GXD_AlleleGenotype (cache) table for all Genotypes
-- which are missing
--
-- INPUT:
--	None
--
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


--
-- Name: gxd_removebadgelband(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_removebadgelband() RETURNS void
    LANGUAGE plpgsql
    AS $$

BEGIN

--
-- NAME: GXD_removeBadGelBand
--
-- DESCRIPTION:
--        
-- To delete Gel Bands that are not associated with any Gel Rows
--
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


--
-- Name: gxd_replacegenotype(text, integer, integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.gxd_replacegenotype(v_createdby text, v_refskey integer, v_oldgenotypekey integer, v_newgenotypekey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

BEGIN

--
-- NAME: GXD_replaceGenotype
--
-- DESCRIPTION:
--        
-- To replace old Genotype with new Genotype
-- in GXD_Specimen, GXD_GelLane, GXD_Expression, GXD_Assay
--
-- INPUT:
--      
-- v_createdBy : MGI_User.name
-- v_refsKey : BIB_Refs._Refs_key
-- v_oldGenotypeKey : GXD_Genotype._Genotype_key
-- v_newGenotypeKey : GXD_Genotype._Genotype_key
--
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


--
-- Name: img_image_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.img_image_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: IMG_Image_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INCLUDED RULES:
--	Deletes Thumbnail Images, as needed
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: img_image_insert(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.img_image_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: IMG_Image_insert()
--
-- DESCRIPTOIN:
--
--	this insert trigger will call ACC_assignMGI
--	in order to add a distinct MGI accession id
--	to the NEW object
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

PERFORM ACC_assignMGI(1001, NEW._Image_key, 'Image');

RETURN NEW;

END;
$$;


--
-- Name: img_setpdo(integer, integer, integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.img_setpdo(v_pixid integer, v_xdim integer, v_ydim integer, v_image_key integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_accID acc_accession.accID%TYPE;
v_prefix varchar(4);
v_imageLDB int;
v_imageType int;

BEGIN

--
-- NAME: IMG_setPDO
--
-- DESCRIPTION:
--        
-- adds the PIX id to the MGI Image (ACC_Accession, IMG_Image)
-- set the IMG_Image.xDim, yDim values
--
-- If image_key is valid and a PIX foreign accession number
-- does not already exist for the _Image_key and the PIX: accession
-- ID does not already exist, the new ID is added to ACC_Accession
-- and the x,y dim update the image record.
--
-- ASSUMES:
-- - _LogicalDB_key for "MGI Image Archive" is 19,
-- - _MGIType_key for mgi Image objects is 9
--
-- REQUIRES:
-- - four integer inputs
-- - _Image_key exists
-- - _Image_key is not referenced by an existing PIX:#
-- - PIX:# does not exist (referencing another _Image_key)
--
-- INPUT:
--
-- v_pixID     : ACC_Accession.accID
-- v_xDim      : IMG_Image.xDmin
-- v_yDim      : IMG_Image.yDmin
-- v_image_key : IMG_Image._Image_key
--
-- RETURNS:
--	VOID

v_prefix := 'PIX:'; 
v_imageLDB := 19;
v_imageType := 9;

IF v_pixID IS NULL OR v_image_key IS NULL OR v_xDim IS NULL OR v_yDim IS NULL
THEN
	RAISE EXCEPTION E'IMG_setPDO: All four arguments must be non-NULL.';
	RETURN;
ELSE
	v_accID := v_prefix || v_pixID;
END IF;

-- ck for missing image rec
IF NOT EXISTS (SELECT 1 FROM IMG_Image WHERE _Image_key = v_image_key)
THEN
	RAISE EXCEPTION E'IMG_setPDO: % _Image_key does not exist.', v_image_key;
	RETURN;
END IF;

-- check that this PIX:# does not already exist
IF EXISTS (SELECT 1 FROM ACC_Accession 
   WHERE accID = v_accID AND _MGIType_key = v_imageType
   AND _LogicalDB_key = v_imageLDB 
   )
THEN
	RAISE EXCEPTION E'IMG_setPDO: % accession already exists.', v_accID;
	RETURN;
END IF;

-- check that image record is not referenced by another PIX:#
IF EXISTS (SELECT 1 FROM ACC_Accession
   WHERE _Object_key = v_image_key AND prefixPart = v_prefix
   AND _LogicalDB_key = v_imageLDB AND _MGIType_key = v_imageType
   )
THEN
	RAISE EXCEPTION E'IMG_setPDO: A PIX: accession already exists for _Image_key %.', v_image_key;
	RETURN;
END IF;

-- insert the new PIX: accession record 
PERFORM ACC_insert (1001, v_image_key, v_accID, v_imageLDB, 'Image', -1, 1, 1);

IF NOT FOUND
THEN
	RAISE EXCEPTION E'IMG_setPDO: ACC_insert failed.';
	RETURN;
END IF;

-- set the image dimensions 
UPDATE IMG_Image 
SET xDim = v_xDim, yDim = v_yDim
WHERE _Image_key = v_image_key
;

IF NOT FOUND
THEN
        RAISE EXCEPTION E'IMG_setPDO: Update x,y Dimensions failed.';
        RETURN;
END IF;

END;
$$;


--
-- Name: mgi_addsetmember(integer, integer, integer, text, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.mgi_addsetmember(v_setkey integer, v_objectkey integer, v_userkey integer, v_label text, v_stagekey integer DEFAULT 0) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_newSetMemberKey int;
v_newSeqKey int;
v_newSetMemberEmapaKey int;

BEGIN

--
-- NAME: MGI_addSetMember
--
-- DESCRIPTION:
--
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
--
-- RETURNS:
--	VOID
--

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


--
-- Name: mgi_checkemapaclipboard(integer); Type: FUNCTION; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_cleannote(integer); Type: FUNCTION; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_deleteemapaclipboarddups(integer); Type: FUNCTION; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_insertreferenceassoc(integer, integer, integer, integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

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

--
-- if the v_refsKey does not have a J:, then add it
--
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


--
-- Name: mgi_insertsynonym(integer, integer, integer, integer, text, integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_mergerelationship(integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.mgi_mergerelationship(v_oldkey integer, v_newkey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

BEGIN

--
-- Participant relationship (any category)
--
-- _Objec_key_1 anything else
-- _Objec_key_2 is Marker (_MGIType_key = 2)
--

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

--
-- Organizer relationship only (1001:'interacts_with', 1002:'clsuter_has_member')
--
-- checking where:
--
-- _Objec_key_1 is Marker (_MGIType_key = 2)
-- _Objec_key_2 is Marker (_MGIType_key = 2)
--

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


--
-- Name: mgi_organism_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.mgi_organism_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: MGI_Organism_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: mgi_organism_insert(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.mgi_organism_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: MGI_Organism_insert()
--
-- DESCRIPTOIN:
--
--	this insert trigger will call ACC_assignMGI
--	in order to add a distinct MGI accession id
--	to the NEW object
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

PERFORM ACC_assignMGI(1001, NEW._Organism_key, 'Organism');

RETURN NEW;

END;
$$;


--
-- Name: mgi_processnote(integer, integer, integer, integer, integer, text); Type: FUNCTION; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_reference_assoc_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.mgi_reference_assoc_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: MGI_Reference_Assoc_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: mgi_reference_assoc_insert(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.mgi_reference_assoc_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$

BEGIN

--
-- NAME: MGI_Reference_Assoc_insert()
--
-- DESCRIPTION:
--
--      1) if the Reference is for an Allele and exists as group = 'AP'
--              in BIB_Workflow_Status, and the status is not 'Full-coded'
--         then
--              set existing BIB_Workflow_Status.isCurrent = 0
--              add new BIB_Workflow_Status._Status_key = 'Indexed'
--
-- changes to this trigger require changes to procedure/BIB_updateWFStatusAP_create.object
--
-- INPUT:
--      none
--
-- RETURNS:
--      NEW
--

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


--
-- Name: mgi_relationship_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.mgi_relationship_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: MGI_Relationship_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

DELETE FROM MGI_Note m
WHERE m._Object_key = OLD._Relationship_key
AND m._MGIType_key = 40
;

RETURN NEW;

END;
$$;


--
-- Name: mgi_resetageminmax(text, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.mgi_resetageminmax(v_table text, v_key integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_pkey int; 	/* primary key of records to UPDATE */
v_age prb_source.age%TYPE;
v_ageMin numeric;
v_ageMax numeric;
age_cursor refcursor;

BEGIN

-- Update the Age Min, Age Max values

IF (v_table = 'GXD_Expression')
THEN
	OPEN age_cursor FOR
	SELECT _Expression_key, age
	FROM GXD_Expression
	WHERE _Expression_key = v_key;

ELSIF (v_table = 'GXD_GelLane')
THEN
	OPEN age_cursor FOR
	SELECT _GelLane_key, age
	FROM GXD_GelLane
	WHERE _GelLane_key = v_key;

ELSIF (v_table = 'GXD_Specimen')
THEN
	OPEN age_cursor FOR
	SELECT _Specimen_key, age
	FROM GXD_Specimen
	WHERE _Specimen_key = v_key;

ELSIF (v_table = 'PRB_Source')
THEN
	OPEN age_cursor FOR
	SELECT _Source_key, age
	FROM PRB_Source
	WHERE _Source_key = v_key;

ELSIF (v_table = 'GXD_HTSample')
THEN
	OPEN age_cursor FOR
	SELECT _Sample_key, age
	FROM GXD_HTSample
	WHERE _Sample_key = v_key;

ELSE
	RETURN;

END IF;

LOOP
	FETCH age_cursor INTO v_pkey, v_age;
	EXIT WHEN NOT FOUND;

	-- see PRB_ageMinMex for exceptions
	SELECT * FROM PRB_ageMinMax(v_age) into v_ageMin, v_ageMax;

        -- no ageMin/ageMax null values
        -- commented out; there are dependcies upstream that are expecting null
        IF (v_ageMin is null)
        THEN
                v_ageMin := -1;
                v_ageMax := -1;
        END IF;

	IF (v_table = 'GXD_Expression')
	THEN
		UPDATE GXD_Expression 
		SET ageMin = v_ageMin, ageMax = v_ageMax WHERE _Expression_key = v_pkey;

	ELSIF (v_table = 'GXD_GelLane')
	THEN
		UPDATE GXD_GelLane 
		SET ageMin = v_ageMin, ageMax = v_ageMax WHERE _GelLane_key = v_pkey;

	ELSIF (v_table = 'GXD_Specimen')
	THEN
		UPDATE GXD_Specimen 
		SET ageMin = v_ageMin, ageMax = v_ageMax WHERE _Specimen_key = v_pkey;

	ELSIF (v_table = 'PRB_Source')
	THEN
		UPDATE PRB_Source 
		SET ageMin = v_ageMin, ageMax = v_ageMax WHERE _Source_key = v_pkey;

	ELSIF (v_table = 'GXD_HTSample')
	THEN
		UPDATE GXD_HTSample
		SET ageMin = v_ageMin, ageMax = v_ageMax WHERE _Sample_key = v_pkey;

	END IF;


END LOOP;

CLOSE age_cursor;

RETURN;

END;
$$;


--
-- Name: mgi_resetsequencenum(text, integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_setmember_emapa_insert(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.mgi_setmember_emapa_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: MGI_SetMember_EMAPA_insert()
--
-- DESCRIPTOIN:
--
--	this insert trigger will call MGI_checkEMAPAClipboard
--		and throw an Exception if the NEW
--		record is invalid
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

PERFORM MGI_checkEMAPAClipboard(NEW._setmember_key);
PERFORM MGI_deleteEMAPAClipboardDups(NEW._setmember_key);

RETURN NEW;

END;
$$;


--
-- Name: mgi_setmember_emapa_update(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.mgi_setmember_emapa_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: MGI_SetMember_EMAPA_update()
--
-- DESCRIPTOIN:
--
--	this update trigger will call MGI_checkEMAPAClipboard
--		and throw an Exception if the NEW
--		record is invalid
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

PERFORM MGI_checkEMAPAClipboard(NEW._setmember_key);
PERFORM MGI_deleteEMAPAClipboardDups(NEW._setmember_key);

RETURN NEW;

END;
$$;


--
-- Name: mgi_statistic_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.mgi_statistic_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: MGI_Statistic_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

DELETE FROM MGI_SetMember msm
USING MGI_Set ms
WHERE msm._Object_key = OLD._Statistic_key
AND msm._Set_key = ms._Set_key
AND ms._MGIType_key = 34
;

RETURN NEW;

END;
$$;


--
-- Name: mgi_updatereferenceassoc(integer, integer, integer, integer, integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_updatesetmember(integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.mgi_updatesetmember(v_setmemberkey integer, v_sequencenum integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

BEGIN

--
-- NAME: MGI_updateSetMember
--
-- DESCRIPTION:
--
-- To modify the MGI_SetMember.sequenceNum
--        
-- INPUT:
--      
-- v_setMemberKey   : MGI_SetMember._SetMember_key
-- v_sequenceNum    : MGI_SetMember.sequenceNum
--
-- RETURNS:
--	VOID
--

UPDATE MGI_SetMember
SET sequenceNum = v_sequenceNum, modification_date = now()
WHERE _SetMember_key = v_setMemberKey
;

RETURN;

END;
$$;


--
-- Name: mld_expt_marker_update(); Type: FUNCTION; Schema: mgd; Owner: -
--

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


--
-- Name: mld_expts_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.mld_expts_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: MLD_Expts_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: mld_expts_insert(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.mld_expts_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: MLD_Expts_insert()
--
-- DESCRIPTOIN:
--
--      1) if the Reference is for group = 'QTL'
--              in BIB_Workflow_Status, and the status is not 'Full-coded'
--         then
--              set existing BIB_Workflow_Status.isCurrent = 0
--              add new BIB_Workflow_Status._Status_key = 'Full-coded'
--
--	2) this insert trigger will call ACC_assignMGI
--	   in order to add a distinct MGI accession id
--	   to the NEW object
--
-- changes to this trigger require changes to procedure/BIB_updateWFStatusQTL_create.object
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: mrk_allelewithdrawal(integer, integer, integer, integer, integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.mrk_allelewithdrawal(v_userkey integer, v_oldkey integer, v_newkey integer, v_refskey integer, v_eventreasonkey integer, v_addassynonym integer DEFAULT 1) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_oldSymbol mrk_marker.symbol%TYPE;
v_oldName mrk_marker.name%TYPE;
v_newSymbol  mrk_marker.symbol%TYPE;

BEGIN

--
-- This procedure will process an allele marker withdrawal.
--
-- An allele marker withdrawal requires:
--	a) the "old" marker key
--	b) the "new" marker key
--	c) the reference key
--	d) the event reason key
--

v_oldSymbol := symbol
FROM MRK_Marker 
WHERE _Marker_key = v_oldKey
     AND _Organism_key = 1
     AND _Marker_Status_key = 1
;

v_oldName := name
FROM MRK_Marker 
WHERE _Marker_key = v_oldKey
     AND _Organism_key = 1
     AND _Marker_Status_key = 1
;

v_newSymbol := symbol
FROM MRK_Marker 
WHERE _Marker_key = v_newKey
     AND _Organism_key = 1
     AND _Marker_Status_key = 1
;

IF v_oldSymbol IS NULL
THEN
        RAISE EXCEPTION E'\nMRK_alleleWithdrawal : Invalid Old Symbol Key %', v_oldKey;
        RETURN;
END IF;

IF v_newSymbol IS NULL
THEN
        RAISE EXCEPTION E'\nMRK_alleleWithdrawal : Invalid New Symbol Key %', v_newKey;
        RETURN;
END IF;

PERFORM MRK_mergeWithdrawal (v_userKey, v_oldKey, v_newKey, v_refsKey, 106563607, v_eventReasonKey, v_addAsSynonym);

IF NOT FOUND
THEN
        RAISE EXCEPTION E'\nMRK_alleleWithdrawal : Could not execute allele withdrawal';
        RETURN;
END IF;

RETURN;

END;
$$;


--
-- Name: mrk_cluster_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.mrk_cluster_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: MRK_Cluster_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: mrk_copyhistory(integer, integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.mrk_copyhistory(v_userkey integer, v_oldkey integer, v_newkey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_historyKey int;
v_refsKey int;
v_eventKey int;
v_eventReasonKey int;
v_createdByKey int;
v_modifiedByKey int;
v_name mrk_history.name%TYPE;
v_event_date mrk_history.event_date%TYPE;

BEGIN

-- Copy History of v_oldKey to v_newKey

FOR v_historyKey, v_refsKey, v_eventKey, v_eventReasonKey, v_name, v_event_date IN
SELECT _History_key, _Refs_key, _Marker_Event_key, _Marker_EventReason_key, name, event_date
FROM MRK_History
WHERE _Marker_key = v_oldKey
ORDER BY sequenceNum
LOOP
	PERFORM MRK_insertHistory (v_userKey, v_newKey, v_historyKey, v_refsKey, v_eventKey, 
		v_eventReasonKey, v_name, v_event_date);
END LOOP;

END;
$$;


--
-- Name: mrk_deletewithdrawal(integer, integer, integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.mrk_deletewithdrawal(v_userkey integer, v_oldkey integer, v_refskey integer, v_eventreasonkey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_oldName mrk_marker.name%TYPE;

BEGIN

--
-- This procedure will process a delete withdrawal.
-- A delete marker withdrawal requires:
--	a) the "old" marker key
--	b) a reference key
--

IF EXISTS (SELECT 1 FROM ALL_Allele WHERE _Marker_key = v_oldKey and isWildType = 0)
THEN
        RAISE EXCEPTION E'\nMRK_deleteWithdrawal: Cannot Delete:  Marker is referenced by Allele.';
        RETURN;
END IF;

v_oldName := (SELECT name FROM MRK_Marker
	 WHERE _Organism_key = 1
	 AND _Marker_Status_key = 1
	 AND _Marker_key = v_oldKey);

-- Update Marker info

UPDATE MRK_Marker 
SET name = 'withdrawn', _Marker_Status_key = 2 , 
	cmOffset = -999.0,
	_ModifiedBy_key = v_userKey, modification_date = now() 
WHERE _Marker_key = v_oldKey
;

IF NOT FOUND
THEN
	RAISE EXCEPTION E'MRK_deleteWithdrawal: Could not update marker';
	RETURN;
END IF;

-- Add History line for withdrawal
PERFORM MRK_insertHistory (v_userKey, v_oldKey, v_oldKey, v_refsKey, 106563609, v_eventReasonKey, v_oldName);

IF NOT FOUND
THEN
	RAISE EXCEPTION E'MRK_deleteWithdrawal: Could not add history';
	RETURN;
END IF;

-- Remove old symbol's wild type allele
DELETE FROM ALL_Allele a
USING MRK_Marker m
WHERE m._Marker_key = v_oldKey
AND m._Marker_key = a._Marker_key
AND a.isWildType = 1 
;

-- Remove MGI_Relationships that are annotated to this Marker

CREATE TEMP TABLE toDelete ON COMMIT DROP
AS SELECT r._Relationship_key
FROM MGI_Relationship r, MGI_Relationship_Category c
WHERE r._Object_key_1 = v_oldKey
AND r._Category_key  = c._Category_key
AND c._MGIType_key_1 = 2
UNION
SELECT r._Relationship_key
FROM MGI_Relationship r, MGI_Relationship_Category c
WHERE r._Object_key_2 = v_oldKey
AND r._Category_key  = c._Category_key
AND c._MGIType_key_2 = 2
;

DELETE 
FROM MGI_Relationship
USING toDelete d
WHERE d._Relationship_key = MGI_Relationship._Relationship_key
;

RETURN;

END;
$$;


--
-- Name: mrk_inserthistory(integer, bigint, bigint, integer, integer, integer, text, timestamp without time zone, integer, integer, timestamp without time zone, timestamp without time zone); Type: FUNCTION; Schema: mgd; Owner: -
--

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


--
-- Name: mrk_marker_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.mrk_marker_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: MRK_Marker_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: mrk_marker_insert(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.mrk_marker_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$

DECLARE
v_offset float;

BEGIN

--
-- NAME: MRK_Marker_insert()
--
-- DESCRIPTOIN:
--
--      this insert trigger will call ACC_assignMGI
--      in order to add a distinct MGI accession id
--      to the NEW object
--
-- INPUT:
--      none
--
-- RETURNS:
--      NEW 
--

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


--
-- Name: mrk_marker_update(); Type: FUNCTION; Schema: mgd; Owner: -
--

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

--
-- from: 'reserved' (_Marker_Status_key = 3)
-- to:  'official' (_Marker_Status_key = 1)
--

--
-- and:
--	reference = J:23000
--

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

--
-- and:
--      marker type = 'Gene'
--      and wild-type allele does not exist
--      see other conitionals below
--
-- then create wild-type allele
--

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


--
-- Name: mrk_mergewithdrawal(integer, integer, integer, integer, integer, integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.mrk_mergewithdrawal(v_userkey integer, v_oldkey integer, v_newkey integer, v_refskey integer, v_eventkey integer, v_eventreasonkey integer, v_addassynonym integer DEFAULT 1) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_alleleOf int;
v_assigningRefKey int;
v_synTypeKey int;
v_oldSymbol mrk_marker.symbol%TYPE;
v_oldName mrk_marker.name%TYPE;
v_newSymbol mrk_marker.symbol%TYPE;
v_newChr mrk_marker.chromosome%TYPE;
v_withdrawnName mrk_marker.name%TYPE;
v_cytogeneticOffset mrk_marker.cytogeneticOffset%TYPE;
v_cmOffset mrk_marker.cmOffset%TYPE;
v_alleleSymbol all_allele.symbol%TYPE;

BEGIN

--
-- This procedure will process a merge marker withdrawal.
-- A merge withdrawal is a withdrawal where both the "old" and "new"
-- markers already exist in the database.
--
-- A merge marker withdrawal requires:
--	a) the "old" marker key
--	b) the "new" marker key
--	c) the reference key
--	d) the event key
--	e) the event reason key
--	f) the "add as synonym" flag
--
--

IF v_oldKey = v_newKey
THEN
	RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Cannot Merge a Symbol into itself: %, %', v_oldKey, v_newKey;
	RETURN;
END IF;

v_oldSymbol := symbol
	FROM MRK_Marker 
	WHERE _Marker_key = v_oldKey
     	AND _Organism_key = 1
     	AND _Marker_Status_key = 1;

v_oldName := name
	FROM MRK_Marker 
	WHERE _Marker_key = v_oldKey
     	AND _Organism_key = 1
     	AND _Marker_Status_key = 1;

IF v_oldSymbol IS NULL
THEN
	RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Invalid Old Symbol Key %', v_oldKey;
	RETURN;
END IF;

v_newSymbol := (
	SELECT symbol
	FROM MRK_Marker 
	WHERE _Marker_key = v_newKey
     	AND _Organism_key = 1
     	AND _Marker_Status_key = 1
	);

IF v_newSymbol IS NULL
THEN
	RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Invalid New Symbol Key %', v_newKey;
	RETURN;
END IF;

-- Prevent the merge if the a Marker Detail Clip exists for both symbols

IF EXISTS (SELECT 1 FROM MRK_Notes WHERE _Marker_key = v_oldKey) AND
   EXISTS (SELECT 1 FROM MRK_Notes WHERE _Marker_key = v_newKey)
THEN
        RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Cannot Merge:  both Symbols contain a Marker Detail Clip.';
        RETURN;
END IF;

-- Else, continue....

if v_eventKey = 4
THEN
	v_withdrawnName := 'withdrawn, allele of ' || v_newSymbol;
	v_alleleOf := 1;
ELSE
	v_withdrawnName := 'withdrawn, = ' || v_newSymbol;
	v_alleleOf := 0;
END IF;

--see TR11855 : make sure new term exists before turning this on
--if v_eventKey = 4
--THEN
        -- Update needsReview flag for strains
        --PERFORM PRB_setStrainReview (v_oldKey, NULL, (select _Term_key from VOC_Term where _Vocab_key = 56 and term = 'Needs Review - Chr'));

        --IF NOT FOUND
        --THEN
	        --RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not flag Strain record for needing review';
	        --RETURN;
        --END IF;
--END IF;

-- If new symbol has a chromosome of UN, update the new symbol's chromosome value
-- with the old symbol chromosome value

IF (SELECT chromosome FROM MRK_Marker WHERE _Marker_key = v_newKey) = 'UN'
THEN
	v_newChr := (SELECT chromosome FROM MRK_Marker WHERE _Marker_key = v_oldKey);

	UPDATE MRK_Marker 
	SET chromosome = v_newChr, _ModifiedBy_key = v_userKey, modification_date = now()
	WHERE _Marker_key = v_newKey
	;

	IF NOT FOUND
	THEN
		RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not update new symbol/chromosome';
		RETURN;
	END IF;
END IF;

-- Update cytogenetic/offset values of new symbol

v_cytogeneticOffset := (SELECT cytogeneticOffset FROM MRK_Marker WHERE _Marker_key = v_oldKey);
v_cmOffset := (SELECT cmOffset FROM MRK_Marker WHERE _Marker_key = v_oldKey);
UPDATE MRK_Marker
SET cytogeneticOffset = v_cytogeneticOffset, cmOffset = v_cmOffset
WHERE _Marker_key = v_newKey
;

IF NOT FOUND
THEN
	RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not update cytogenetic/offset values';
	RETURN;
END IF;

-- Update name/cytogenetic/offset of old symbol

UPDATE MRK_Marker 
SET name = v_withdrawnName, cytogeneticOffset = null, _Marker_Status_key = 2, cmOffset = -999.0,
	_ModifiedBy_key = v_userKey, modification_date = now()
WHERE _Marker_key = v_oldKey
;

IF NOT FOUND
THEN
	RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not update name of old symbol : %', v_oldKey;
	RETURN;
END IF;

-- Merge potential duplicate wild type alleles of old and new symbols
-- before converting oldsymbol alleles

PERFORM ALL_mergeWildTypes (v_oldKey, v_newKey, v_oldSymbol, v_newSymbol);

IF NOT FOUND
THEN
	RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not merge wild type alleles';
	RETURN;
END IF;

-- Convert Remaining Alleles

PERFORM ALL_convertAllele (v_userKey, v_oldKey, v_oldSymbol, v_newSymbol, v_alleleOf);

--IF NOT FOUND
--THEN
--	RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not convert alleles';
--	RETURN;
--END IF;

IF v_alleleOf = 1
THEN
	-- If no alleles exist for the old symbol, create a newSymbol<oldSymbol> allele

	IF NOT EXISTS (SELECT 1 FROM ALL_Allele WHERE _Marker_key = v_oldKey)
	THEN
		v_alleleSymbol := v_newSymbol || '<' || v_oldSymbol || '>';

		PERFORM ALL_insertAllele (v_userKey, v_newKey, v_alleleSymbol, v_oldName, 
			v_refsKey, 0, null, null, null, null,
			-1, 'Not Specified', 'Not Specified', 'Approved', v_oldSymbol);

		IF NOT FOUND
		THEN
			RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not insert allele';
			RETURN;
		END IF;
	END IF;
END IF;

-- Merge Marker Feature Relationships (MGI_Relationship)

PERFORM MGI_mergeRelationship (v_oldKey, v_newKey);

IF NOT FOUND
THEN
	RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not merge Marker Feature Relationships';
	RETURN;
END IF;

-- Update current symbols

UPDATE MRK_Current 
SET _Current_key = v_newKey 
WHERE _Marker_key = v_oldKey
;

IF NOT FOUND
THEN
	RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not update current symbols';
	RETURN;
END IF;

-- Copy History records from old symbol to new symbol

PERFORM MRK_copyHistory (v_userKey, v_oldKey, v_newKey);

IF NOT FOUND
THEN
	RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not copy history records';
	RETURN;
END IF;

-- Insert history record for withdrawal

PERFORM MRK_insertHistory (v_userKey, v_newKey, v_oldKey, v_refsKey, v_eventKey, v_eventReasonKey, v_oldName);

IF NOT FOUND
THEN
	RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not create history record';
	RETURN;
END IF;

-- Remove history records from old symbol

DELETE FROM MRK_History WHERE _Marker_key = v_oldKey;

-- Insert withdrawn symbol into Synonym table
-- Use assigning reference
IF v_addAsSynonym = 1
THEN
	v_assigningRefKey := (
		SELECT DISTINCT _Refs_key 
		FROM MRK_History_View
		WHERE _Marker_key = v_newKey
		AND history = v_oldSymbol
		AND _Marker_Event_key = 1
		ORDER BY _Refs_key
		LIMIT 1
		);

	-- If oldSymbol is a Riken symbol (ends with 'Rik'), 
	-- then synonym type = 'similar' else synonym type = 'exact'

	IF EXISTS (SELECT 1
		   FROM MRK_Marker 
                   WHERE _Marker_key = v_oldKey
                        AND _Organism_key = 1
                        AND _Marker_Status_key = 1
		        AND symbol like '%Rik'
		   )
	THEN
	    v_synTypeKey := 1005;
        ELSE
	    v_synTypeKey := 1004;
        END IF;

	PERFORM MGI_insertSynonym (v_userKey, v_newKey, 2, v_synTypeKey, v_oldSymbol, v_assigningRefKey, 1);

	IF NOT FOUND
	THEN
		RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not add synonym';
		RETURN;
	END IF;
END IF;

-- Remove old symbol's wild type allele
DELETE FROM ALL_Allele a
USING MRK_Marker m
WHERE m._Marker_key = v_oldKey
AND m._Marker_key = a._Marker_key
AND a.isWildType = 1
;

-- Update keys from old key to new key

PERFORM MRK_updateKeys (v_oldKey, v_newKey);

IF NOT FOUND
THEN
	RAISE EXCEPTION E'\nMRK_mergeWithdrawal: Could not update keys';
	RETURN;
END IF;

END;
$$;


--
-- Name: mrk_reloadlocation(integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.mrk_reloadlocation(v_markerkey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_markerTypeKey int;
v_organismKey int;
v_sequenceNum int;
v_chromosome mrk_marker.chromosome%TYPE;
v_cytogeneticOffset mrk_marker.cytogeneticOffset%TYPE;
v_cmoffset mrk_marker.cmoffset%TYPE;
v_startCoordinate seq_coord_cache.startCoordinate%TYPE;
v_endCoordinate seq_coord_cache.endCoordinate%TYPE;
v_strand seq_coord_cache.strand%TYPE;
v_mapUnits voc_term.term%TYPE;
v_provider map_coord_collection.abbreviation%TYPE;
v_version seq_coord_cache.version%TYPE;
v_genomicChromosome seq_coord_cache.chromosome%TYPE;

BEGIN

DELETE FROM MRK_Location_Cache where _Marker_key = v_markerKey;

FOR v_chromosome, v_cytogeneticOffset, v_markerTypeKey, v_organismKey, v_cmoffset, 
    v_sequenceNum, v_startCoordinate, v_endCoordinate, v_strand, v_mapUnits, 
    v_provider, v_version, v_genomicChromosome
IN
select m.chromosome, m.cytogeneticOffset, m._Marker_Type_key, m._Organism_key, m.cmOffset,
c.sequenceNum,
f.startCoordinate, f.endCoordinate, f.strand, u.term, cl.abbreviation, cc.version, gc.chromosome as genomicChromosome
from MRK_Marker m, MRK_Chromosome c,
MAP_Coord_Collection cl, MAP_Coordinate cc, MAP_Coord_Feature f, VOC_Term u, MRK_Chromosome gc
where m._Marker_key = v_markerKey
and m._Organism_key = c._Organism_key
and m.chromosome = c.chromosome
and m._Marker_key = f._Object_key
and f._MGIType_key = 2
and f._Map_key = cc._Map_key
and cc._Collection_key = cl._Collection_key
and cc._Units_key = u._Term_key
and cc._Object_key = gc._Chromosome_key

UNION
select m.chromosome, m.cytogeneticOffset, m._Marker_Type_key, m._Organism_key, m.cmOffset,
ch.sequenceNum,
c.startCoordinate, c.endCoordinate, c.strand, c.mapUnits, cl.abbreviation, c.version, c.chromosome as genomicChromosome
from MRK_Marker m, MRK_Chromosome ch, 
     SEQ_Marker_Cache mc, SEQ_Coord_Cache c, MAP_Coordinate cc,
     MAP_Coord_Collection cl
where m._Marker_key = v_markerKey
and m._Organism_key = ch._Organism_key 
and m.chromosome = ch.chromosome 
and m._Marker_key = mc._Marker_key
and mc._Qualifier_key = 615419
and mc._Sequence_key = c._Sequence_key
and c._Map_key = cc._Map_key
and cc._Collection_key = cl._Collection_key

UNION
select m.chromosome, m.cytogeneticOffset, m._Marker_Type_key, m._Organism_key, m.cmOffset,
c.sequenceNum,NULL,NULL,NULL,NULL,NULL,NULL,NULL
from MRK_Marker m, MRK_Chromosome c
where m._Marker_key = v_markerKey
and m._Organism_key = c._Organism_key
and m.chromosome = c.chromosome
and not exists (select 1 from SEQ_Marker_Cache mc, SEQ_Coord_Cache c
where m._Marker_key = mc._Marker_key
and mc._Qualifier_key = 615419
and mc._Sequence_key = c._Sequence_key)
and not exists (select 1 from MAP_Coord_Feature f
where m._Marker_key = f._Object_key
and f._MGIType_key = 2)

UNION
select m.chromosome, m.cytogeneticOffset, m._Marker_Type_key, m._Organism_key, m.cmOffset,
c.sequenceNum,NULL,NULL,NULL,NULL,NULL,NULL,NULL
from MRK_Marker m, MRK_Chromosome c
where m._Marker_key = v_markerKey
and m._Organism_key = c._Organism_key
and m.chromosome = c.chromosome
and not exists (select 1 from SEQ_Marker_Cache mc, SEQ_Coord_Cache c
where m._Marker_key = mc._Marker_key
and mc._Qualifier_key = 615419
and mc._Sequence_key = c._Sequence_key)
and not exists (select 1 from MAP_Coord_Feature f
where m._Marker_key = f._Object_key
and f._MGIType_key = 2)

LOOP
	INSERT INTO MRK_Location_Cache
	(_Marker_key, _Marker_Type_key, _Organism_key, chromosome, sequenceNum, cytogeneticOffset, 
		cmoffset, genomicChromosome, startCoordinate, endCoordinate, strand, mapUnits, provider, version)
	VALUES(v_markerKey, v_markerTypeKey, v_organismKey, v_chromosome, v_sequenceNum, v_cytogeneticOffset, 
		v_cmoffset, v_genomicChromosome, v_startCoordinate, v_endCoordinate, v_strand, v_mapUnits, v_provider, v_version)
	;

        -- only process 1st value
        EXIT;

END LOOP;

END;
$$;


--
-- Name: mrk_reloadreference(integer); Type: FUNCTION; Schema: mgd; Owner: -
--

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


--
-- Name: mrk_simplewithdrawal(integer, integer, integer, integer, text, text, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.mrk_simplewithdrawal(v_userkey integer, v_oldkey integer, v_refskey integer, v_eventreasonkey integer, v_newsymbol text, v_newname text, v_addassynonym integer DEFAULT 1) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_newKey int;
v_withdrawnName mrk_marker.name%TYPE;
v_savesymbol mrk_marker.symbol%TYPE;

BEGIN

--
-- This procedure will process a simple marker withdrawal.
-- A simple marker withdrawal requires:
--	a) the "old" marker key
--	b) the reference key
--	c) the event reason key
--	c) the "new" marker symbol which does not already exist
--	d) the "new" marker name
--	e) the "add as synonym" flag
-- 
--  Since the server is not case-sensitive, the caller is
--  responsible for making sure the new symbol is unique and correct.
-- 

v_withdrawnName := 'withdrawn, = ' || v_newSymbol;
v_newKey := nextval('mrk_marker_seq') as mrk_marker_seq;

CREATE TEMP TABLE mrk_tmp ON COMMIT DROP
AS SELECT distinct m.symbol as oldSymbol, m.name as oldName, h._Refs_key as assigningRefKey
FROM MRK_Marker m, MRK_History h
WHERE m._Organism_key = 1
     AND m._Marker_Status_key = 1
     AND m._Marker_key = v_oldKey 
     AND m._Marker_key = h._Marker_key
     AND h._History_key = v_oldKey
     AND h._Marker_Event_key = 106563604
;

IF (SELECT count(*) FROM mrk_tmp) = 0
THEN
	RAISE EXCEPTION E'\nMRK_simpleWithdrawal: Marker History is missing "assigned" event (%)', v_newSymbol;
	RETURN;
END IF;

-- Check for duplicates; exclude cytogenetic markers

IF EXISTS (SELECT * FROM MRK_Marker 
	WHERE _Organism_key = 1 
	AND _Marker_Status_key = 1
	AND _Marker_Type_key != 3
	AND symbol = v_newSymbol)
THEN
	RAISE EXCEPTION E'\nMRK_simpleWithdrawal: Duplicate Symbol (%)', v_newSymbol;
	RETURN;
END IF;

v_savesymbol := symbol FROM MRK_Marker WHERE _Marker_key = v_oldKey;

-- Create a new marker record using the old marker record as the template
INSERT INTO MRK_Marker 
(_Marker_key, _Organism_key, _Marker_Type_key, _Marker_Status_key, symbol, name, chromosome, cmOffset, _CreatedBy_key, _ModifiedBy_key)
SELECT v_newKey, _Organism_key, _Marker_Type_key, 2, symbol||'_tmp', v_withdrawnName, chromosome, cmOffset, v_userKey, v_userKey
FROM MRK_Marker
WHERE _Marker_key = v_oldKey
;

IF NOT FOUND
THEN
	RAISE EXCEPTION E'\nMRK_simpleWithdrawal: Could not add marker (%)', v_newSymbol;
	RETURN;
END IF;

-- Update the Current marker of the new marker

UPDATE MRK_Current 
SET _Current_key = v_oldKey 
WHERE _Current_key = v_newKey;

-- handled by trigger
--INSERT INTO MRK_Current VALUES (v_oldKey, v_newKey, now(), now());

-- Update old marker record with new symbol and name values
UPDATE MRK_Marker 
SET symbol = v_newSymbol, name = v_newName, _ModifiedBy_key = v_userKey, modification_date = now()
WHERE _Marker_key = v_oldKey;

-- Update old marker record with old symbol (remove '_tmp')
UPDATE MRK_Marker 
SET symbol = v_savesymbol
WHERE _Marker_key = v_newKey;

-- Update history lines
UPDATE MRK_History 
SET _History_key = v_newKey 
WHERE _Marker_key = v_oldKey 
AND _History_key = v_oldKey;

-- Add History line for withdrawal
PERFORM MRK_insertHistory (v_userKey, v_oldKey, v_newKey, v_refsKey, 106563605, v_eventReasonKey, 
	(select oldName from mrk_tmp));

IF NOT FOUND
THEN
	RAISE EXCEPTION E'\nMRK_simpleWithdrawal: Could not add history (withdrawal)';
	RETURN;
END IF;

-- Add History line for assignment
PERFORM MRK_insertHistory (v_userKey, v_oldKey, v_oldKey, v_refsKey, 106563604, v_eventReasonKey, v_newName);

IF NOT FOUND
THEN
	RAISE EXCEPTION E'\nMRK_simpleWithdrawal: Could not add history (assignment)';
	RETURN;
END IF;

-- If marker type = 'gene' (1) and no wild type allele exists for new symbol, create it

--IF (SELECT _Marker_Type_key FROM MRK_Marker WHERE _Marker_key = v_newKey) = 1
	--AND NOT EXISTS (SELECT 1 FROM ALL_Allele WHERE _Marker_key = v_oldKey AND isWildType = 1)
--THEN
	--PERFORM ALL_createWildType (v_userKey, v_oldKey, v_newSymbol);

	--IF NOT FOUND
	--THEN
		--RAISE EXCEPTION E'\nMRK_simpleWithdrawal: Could not add wild type allele';
		--RETURN;
	--END IF;
--END IF;

-- Convert Alleles, if necessary

PERFORM ALL_convertAllele (v_userKey, v_oldKey, (select oldSymbol from mrk_tmp), v_newSymbol);

IF NOT FOUND
THEN
	RAISE EXCEPTION E'\nMRK_simpleWithdrawal: Could not call ALL_convertAllele';
	RETURN;
END IF;

-- Insert withdrawn symbol into Synonym table
-- Use assigning reference

IF v_addAsSynonym = 1
THEN
	PERFORM MGI_insertSynonym (v_userKey, v_oldKey, 2, 1004, 
		(select oldSymbol from mrk_tmp), 
		(select assigningRefKey from mrk_tmp), 1);

	IF NOT FOUND
	THEN
		RAISE EXCEPTION E'\nMRK_simpleWithdrawal: Could not add synonym';
		RETURN;
	END IF;
END IF;

-- Update needsReview flag for strains
PERFORM PRB_setStrainReview (v_oldKey);

IF NOT FOUND
THEN
	RAISE EXCEPTION E'\nMRK_simpleWithdrawal: Could not flag Strain record for needing review';
	RETURN;
END IF;

RETURN;

END;
$$;


--
-- Name: mrk_strainmarker_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.mrk_strainmarker_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: MRK_StrainMarker_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: mrk_updatekeys(integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.mrk_updatekeys(v_oldkey integer, v_newkey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_userKey int;

BEGIN

-- Executed during merge withdrawal process

--
-- Set the preferred bit to 0 for all MGI Acc# brought over from old symbol if
-- the new symbol already contains a preferred MGI Acc#.
-- Associate all Accession numbers w/ new symbol.
--

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

--
-- remove any Accession records belonging to old marker
-- which already exist for new marker
-- that is, remove duplicates before updating keys
--

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
--

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


--
-- Name: prb_ageminmax(text); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.prb_ageminmax(v_age text, OUT v_agemin numeric, OUT v_agemax numeric) RETURNS record
    LANGUAGE plpgsql
    AS $$

DECLARE
saveAge prb_source.age%TYPE;
stem prb_source.age%TYPE;
timeUnit varchar(25);
timeRange varchar(25);
i integer;
idx integer;
idx2 integer;
minValue numeric;
maxValue numeric;

BEGIN

saveAge := v_age;

IF v_age = 'Not Specified' 
	or v_age = 'Not Applicable' 
	or v_age = 'Not Loaded'
	or v_age = 'Not Resolved'
THEN
	v_ageMin := -1.0;
	v_ageMax := -1.0;

ELSIF v_age = 'embryonic'
THEN
	v_ageMin := 0.0;
	v_ageMax := 21.0;

ELSIF v_age = 'postnatal'
THEN
	v_ageMin := 21.01;
	v_ageMax := 1846.0;

ELSIF v_age = 'postnatal newborn'
THEN
	v_ageMin := 21.01;
	v_ageMax := 25.0;

ELSIF v_age = 'postnatal adult'
THEN
	v_ageMin := 42.01;
	v_ageMax := 1846.0;

ELSIF v_age = 'postnatal day'
	or v_age = 'postnatal week'
	or v_age = 'postnatal month'
	or v_age = 'postnatal year'
	or v_age = 'embryonic day'
THEN
        RAISE NOTICE E'Invalid Age Value: "%"\n', saveAge;
	RETURN;

-- parse the age into 3 parts:          
--     stem (embryonic, postnatal),     
--     time unit (day, week, month year) 
--     time range (x   x,y,z   x-y)     

ELSE

    minValue := 5000.0;
    maxValue := -1.0;
    i := 0;

    WHILE v_age IS NOT NULL AND i < 3 LOOP

        idx := position(' ' in v_age);

        IF idx > 0 THEN
            IF i = 0 THEN
                stem := substring(v_age, 1, idx - 1);
            ELSIF i = 1 THEN
                timeUnit := substring(v_age, 1, idx - 1);
            ELSIF i = 1 THEN
                timeRange := substring(v_age, 1, char_length(v_age));
	    END IF;

            v_age := substring(v_age, idx + 1, char_length(v_age));
        ELSE
            timeRange := substring(v_age, 1, char_length(v_age));
            v_age := null;
        END IF;

        i := i + 1;

    END LOOP; -- WHILE v_age != null...

    -- once the stem, time unit and time range have been parsed,
    -- determine the format of the time range and process accordingly.

    -- format is:                
    --     embryonic day x,y,z    
    --     embryonic day x-y,z     
    --     embryonic day x,y-z       
    --                               
    -- we assume that x is the min and z is the max 
    --                                              
    -- NOTE:  there are very fiew of these cases (29 as of 07/23/2003) 

    IF position(',' in timeRange) > 0 THEN

        WHILE timeRange IS NOT NULL LOOP
            idx := position(',' in timeRange);
            idx2 := position('-' in timeRange);

            IF idx2 > 0 THEN

                -- the x-y of an "x-y,z..." 

                IF idx > idx2 THEN
                    IF minValue > substring(timeRange, 1, idx2 - 1)::NUMERIC THEN
                        minValue := substring(timeRange, 1, idx2 - 1)::NUMERIC;
		    END IF;
                    IF maxValue < substring(timeRange, idx2 + 1, idx - idx2 - 1)::NUMERIC THEN
                        maxValue := substring(timeRange, idx2 + 1, idx - idx2 - 1)::NUMERIC;
		    END IF;

                -- more timeRanges (more commas)
                ELSIF idx > 0 THEN
                    IF minValue > substring(timeRange, 1, idx - 1)::NUMERIC THEN
                        minValue := substring(timeRange, 1, idx - 1)::NUMERIC;
		    END IF;
                    IF maxValue < convert(numeric, substring(timeRange, 1, idx - 1)) THEN
                        maxValue := substring(timeRange, 1, idx - 1)::NUMERIC;
		    END IF;

                -- last timeRange
                ELSE
                    IF minValue > substring(timeRange, 1, idx2 - 1)::NUMERIC THEN
                        minValue := substring(timeRange, 1, idx2 - 1)::NUMERIC;
		    END IF;
                    IF maxValue < substring(timeRange, idx2 + 1, char_length(timeRange))::NUMERIC THEN
                        maxValue := substring(timeRange, idx2 + 1, char_length(timeRange))::NUMERIC;
		    END IF;

                END IF;

            ELSE

                -- more timeRanges
                IF idx > 0 THEN
                    IF minValue > substring(timeRange, 1, idx - 1)::NUMERIC THEN
                        minValue := substring(timeRange, 1, idx - 1)::NUMERIC;
                    END IF;
                    IF maxValue < substring(timeRange, 1, idx - 1)::NUMERIC THEN
                        maxValue := substring(timeRange, 1, idx - 1)::NUMERIC;
                    END IF;

                -- last timeRange
                ELSE
                    IF minValue > timeRange::NUMERIC THEN
                        minValue := timeRange::NUMERIC;
                    END IF;
                    IF maxValue < timeRange::NUMERIC THEN
                        maxValue := timeRange::NUMERIC;
		    END IF;
                END IF;
            END IF;

            IF position(',' in timeRange) = 0 THEN
                timeRange := null;
            ELSE
                timeRange := substring(timeRange, idx + 1, char_length(timeRange));
            END IF;

        END LOOP;

    ELSE

        -- format is:                     
        --     embryonic day x-y          

        idx := position('-' in timeRange);

        IF idx > 0 THEN
            minValue := substring(timeRange, 1, idx - 1)::NUMERIC;
            maxValue := substring(timeRange, idx + 1, char_length(timeRange))::NUMERIC;

        -- format is:                    
        --     embryonic day x          

        ELSE
            minValue := timeRange::NUMERIC;
            maxValue := timeRange::NUMERIC;
        END IF;

    END IF; -- end IF position(',' in timeRange) > 0 THEN

    IF minValue is null or maxValue is null THEN
        RETURN;
    END IF;

    -- multiply postnatal values according to time unit

    IF stem = 'postnatal' THEN

        IF timeUnit = 'day' THEN
            v_ageMin := minValue + 21.01;
            v_ageMax := maxValue + 21.01;

        ELSEIF timeUnit = 'week' THEN
            v_ageMin := minValue * 7 + 21.01;
            v_ageMax := maxValue * 7 + 21.01;

        ELSEIF timeUnit = 'month' THEN
            v_ageMin := minValue * 30 + 21.01;
            v_ageMax := maxValue * 30 + 21.01;

        ELSEIF timeUnit = 'year' THEN
            v_ageMin := minValue * 365 + 21.01;
            v_ageMax := maxValue * 365 + 21.01;
        END IF;

    ELSE
        v_ageMin := minValue;
        v_ageMax := maxValue;
    END IF;

END IF; -- final

IF stem = 'Not' AND v_ageMin >= 0
THEN
    RAISE NOTICE E'Invalid Age Value: "%"\n', saveAge;
    RETURN;
END IF;

IF stem = 'embryonic' AND timeUnit IS NULL AND timeRange IS NOT NULL
THEN
    RAISE NOTICE E'Invalid Age Value: "%"\n', saveAge;
    RETURN;
END IF;

IF v_ageMin IS NULL AND v_ageMax IS NULL THEN
    RAISE NOTICE E'Invalid Age Value: "%"\n', saveAge;
    RETURN;
END IF;

RETURN;

END;
$$;


--
-- Name: prb_getstrainbyreference(integer); Type: FUNCTION; Schema: mgd; Owner: -
--

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


--
-- Name: prb_getstraindatasets(integer); Type: FUNCTION; Schema: mgd; Owner: -
--

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


--
-- Name: prb_getstrainreferences(integer); Type: FUNCTION; Schema: mgd; Owner: -
--

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


--
-- Name: prb_insertreference(integer, integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

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


--
-- Name: prb_marker_insert(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.prb_marker_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: PRB_Marker_insert()
--
-- DESCRIPTOIN:
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: prb_marker_update(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.prb_marker_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: PRB_Marker_update()
--
-- DESCRIPTOIN:
--
--	a) Relationship rules : see below for details
--	b) propagate Marker changes to PRB_RFLV
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: prb_mergestrain(integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.prb_mergestrain(v_oldstrainkey integer, v_newstrainkey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE

v_alleleKey int;
v_probe text;
v_jnum text;
v_strainAttributesType int;
v_oldNeedsReviewSum int;
v_newNeedsReviewSum int;
v_jaxRegistryNew acc_accession.accID%TYPE;
v_jaxRegistryOld acc_accession.accID%TYPE;
v_strainmergeKey int;

v_noteTypeKey int;
v_note1 text;
v_note2 text;

v_nextKey int;
v_synTypeKey int;
v_nextSeqKey int;

BEGIN

-- Update old Strain key to new Strain key
-- in all relevant tables which contain a Strain key.
-- When finished, remove the Strain record for the old Strain key.

IF v_oldStrainKey = v_newStrainKey
THEN
	RAISE EXCEPTION E'PRB_mergeStrain: Cannot merge a Strain into itself.';
	RETURN;
END IF;

-- Check for valid merge conditions
-- disallowed:
--	private -> public
--      public -> private
--      Standard -> Non Standard 

IF (SELECT private FROM PRB_Strain WHERE _Strain_key = v_oldStrainKey) = 1
   AND
   (SELECT private FROM PRB_Strain WHERE _Strain_key = v_newStrainKey) = 0
THEN
	RAISE EXCEPTION E'PRB_mergeStrain: Cannot merge Private Strain into Public Strain.';
	RETURN;
END IF;

IF (SELECT private FROM PRB_Strain WHERE _Strain_key = v_oldStrainKey) = 0
   AND
   (SELECT private FROM PRB_Strain WHERE _Strain_key = v_newStrainKey) = 1
THEN
	RAISE EXCEPTION E'PRB_mergeStrain: Cannot merge Public Strain into Private Strain';
	RETURN;
END IF;

IF (SELECT standard FROM PRB_Strain WHERE _Strain_key = v_oldStrainKey) = 1
   AND
   (SELECT standard FROM PRB_Strain WHERE _Strain_key = v_newStrainKey) = 0
THEN
	RAISE EXCEPTION E'PRB_mergeStrain: Cannot merge Standard Strain into Non-Standard Strain';
	RETURN;
END IF;

-- Check for potential duplicate Probe RFLV Entries

FOR v_alleleKey IN
SELECT DISTINCT _Allele_key
FROM PRB_Allele_Strain
WHERE _Strain_key in (v_oldStrainKey, v_newStrainkey)
GROUP by _Allele_key having count(*) > 1
LOOP
	SELECT p.name as v_probe, b.accID as v_jnum
	FROM PRB_Allele a, PRB_RFLV v, PRB_Reference  r, PRB_Probe p, BIB_Acc_View b
	WHERE a._Allele_key = v_alleleKey and
	      a._RFLV_key = v._RFLV_key and
	      v._Reference_key = r._Reference_key and
	      r._Probe_key = p._Probe_key and
	      r._Refs_key = b._Object_key and
	      b.prefixPart = 'J:' and
	      b._LogicalDB_key = 1
        ;

	RAISE EXCEPTION E'PRB_mergeStrain: This merge would create a duplicate entry for Probe %, %.', v_probe, v_jnum;
	RETURN;
END LOOP;

-- all Strains must have same symbols

IF EXISTS (SELECT m1.* FROM PRB_Strain_Marker m1
           WHERE m1._Strain_key = v_newStrainKey
	   AND NOT EXISTS
	   (SELECT m2.* FROM PRB_Strain_Marker m2
	    WHERE m2._Strain_key = v_oldStrainKey AND
	    m2._Marker_key = m1._Marker_key))
THEN
	RAISE EXCEPTION E'PRB_mergeStrain: Incorrect and Correct Strains must have the same Marker Symbols.';
	RETURN;
END IF;

IF EXISTS (SELECT m1.* FROM PRB_Strain_Marker m1
           WHERE m1._Strain_key = v_oldStrainKey
	   AND NOT EXISTS
	   (SELECT m2.* FROM PRB_Strain_Marker m2
	    WHERE m2._Strain_key = v_newStrainKey AND
	    m2._Marker_key = m1._Marker_key))
THEN
	RAISE EXCEPTION E'PRB_mergeStrain: Incorrect and Correct Strains must have the same Marker Symbols.';
	RETURN;
END IF;

-- both Strains must have the same Strain Attributes

--v_strainAttributesType := _AnnotType_key from VOC_AnnotType WHERE name = 'Strain/Attributes';
--v_strainmergeKey := _User_key from MGI_User WHERE login = 'strainmerge';
v_strainAttributesType := 1009;
v_strainmergeKey := 1400;

IF EXISTS (SELECT m1.* FROM VOC_Annot m1
           WHERE m1._Object_key = v_newStrainKey
	   AND m1._AnnotType_key = v_strainAttributesType
	   AND NOT EXISTS
	   (SELECT m2.* FROM VOC_Annot m2
	    WHERE m2._Object_key = v_oldStrainKey
	    AND m2._AnnotType_key = v_strainAttributesType
	    AND m2._Term_key = m1._Term_key))
THEN
	RAISE EXCEPTION E'PRB_mergeStrain: Incorrect and Correct Strains must have the same Strain Attributes.';
	RETURN;
END IF;

IF EXISTS (SELECT m1.* FROM VOC_Annot m1
           WHERE m1._Object_key = v_oldStrainKey
	   AND m1._AnnotType_key = v_strainAttributesType
	   AND NOT EXISTS
	   (SELECT m2.* FROM VOC_Annot m2
	    WHERE m2._Object_key = v_newStrainKey
	    AND m2._AnnotType_key = v_strainAttributesType
	    AND m2._Term_key = m1._Term_key))
THEN
	RAISE EXCEPTION E'PRB_mergeStrain: Incorrect and Correct Strains must have the same Strain Attributes.';
	RETURN;
END IF;

-- both Strains must have the same Species value

IF (SELECT _Species_key FROM PRB_Strain WHERE _Strain_key = v_newStrainKey) !=
   (SELECT _Species_key FROM PRB_Strain WHERE _Strain_key = v_oldStrainKey)
THEN
	RAISE EXCEPTION E'PRB_mergeStrain: Incorrect and Correct Strains must have the same Species.';
	RETURN;
END IF;

-- both Strains must have the same Needs Review values

v_oldNeedsReviewSum := sum(_Term_key) from PRB_Strain_NeedsReview_View WHERE _Object_key = v_oldStrainKey;
v_newNeedsReviewSum := sum(_Term_key) from PRB_Strain_NeedsReview_View WHERE _Object_key = v_newStrainKey;

IF (v_oldNeedsReviewSum != v_newNeedsReviewSum)
THEN
	RAISE EXCEPTION E'PRB_mergeStrain: Incorrect and Correct Strains must have the same Needs Review values.';
	RETURN;
END IF;

-- JAX Registry - must be equal OR use the one that exists

v_jaxRegistryNew := NULL;
v_jaxRegistryOld := NULL;

IF EXISTS (SELECT accID FROM ACC_Accession 
	   WHERE _Object_key = v_newStrainKey and _LogicalDB_Key = 22 and _MGIType_key = 10)
THEN
	v_jaxRegistryNew := accID FROM ACC_Accession 
		WHERE _Object_key = v_newStrainKey and _LogicalDB_Key = 22 and _MGIType_key = 10;
END IF;

IF EXISTS (SELECT _Accession_key FROM ACC_Accession
           WHERE _Object_key = v_oldStrainKey and _LogicalDB_Key = 22 and _MGIType_key = 10)
THEN
	v_jaxRegistryOld := accID FROM ACC_Accession
		WHERE _Object_key = v_oldStrainKey and _LogicalDB_Key = 22 and _MGIType_key = 10;
END IF;

IF (v_jaxRegistryNew != NULL AND v_jaxRegistryOld != NULL AND v_jaxRegistryNew != v_jaxRegistryOld)
THEN
	RAISE EXCEPTION E'PRB_mergeStrain: Incorrect and Correct Strains must have the same JAX Registry Number.';
	RETURN;
ELSIF (v_jaxRegistryOld != NULL)
THEN
    v_jaxRegistryNew := v_jaxRegistryOld;
END IF;

-- Set the preferred bit to 0 for all MGI Acc# brought over from old strain.

UPDATE ACC_Accession 
SET _Object_key = v_newStrainKey, preferred = 0
WHERE _LogicalDB_key = 1 and _MGIType_key = 10 and _Object_key = v_oldStrainKey
;

-- remove any Accession records belonging to old strain 
-- which already exist for new strain 
-- that is, remove duplicates before updating keys 

DELETE FROM ACC_Accession old
USING ACC_Accession new
WHERE old._MGIType_key = 10
AND old._Object_key = v_oldStrainKey
AND old.accID = new.accID
AND old._LogicalDB_key = new._LogicalDB_key
AND new._Object_key = v_newStrainKey
AND new._MGIType_key = 10
;

UPDATE ACC_Accession 
SET _Object_key = v_newStrainKey
WHERE _MGIType_key = 10 and _Object_key = v_oldStrainKey
;

UPDATE ALL_Allele
SET _Strain_key = v_newStrainKey, _ModifiedBy_key = v_strainmergeKey, modification_date = now()
WHERE _Strain_key = v_oldStrainKey
;

UPDATE ALL_CellLine
SET _Strain_key = v_newStrainKey
WHERE _Strain_key = v_oldStrainKey
;

UPDATE PRB_Source
SET _Strain_key = v_newStrainKey, _ModifiedBy_key = v_strainmergeKey, modification_date = now()
WHERE _Strain_key = v_oldStrainKey
;

UPDATE PRB_Allele_Strain
SET _Strain_key = v_newStrainKey, _ModifiedBy_key = v_strainmergeKey, modification_date = now()
WHERE _Strain_key = v_oldStrainKey
;

UPDATE MLD_FISH
SET _Strain_key = v_newStrainKey
WHERE _Strain_key = v_oldStrainKey
;

UPDATE MLD_InSitu
SET _Strain_key = v_newStrainKey
WHERE _Strain_key = v_oldStrainKey
;

UPDATE CRS_Cross
SET _femaleStrain_key = v_newStrainKey
WHERE _femaleStrain_key = v_oldStrainKey
;

UPDATE CRS_Cross
SET _maleStrain_key = v_newStrainKey
WHERE _maleStrain_key = v_oldStrainKey
;

UPDATE CRS_Cross
SET _StrainHO_key = v_newStrainKey
WHERE _StrainHO_key = v_oldStrainKey
;

UPDATE CRS_Cross
SET _StrainHT_key = v_newStrainKey
WHERE _StrainHT_key = v_oldStrainKey
;

UPDATE GXD_Genotype
SET _Strain_key = v_newStrainKey, _ModifiedBy_key = v_strainmergeKey, modification_date = now()
WHERE _Strain_key = v_oldStrainKey
;

UPDATE RI_RISet
SET _Strain_key_1 = v_newStrainKey
WHERE _Strain_key_1 = v_oldStrainKey
;

UPDATE RI_RISet
SET _Strain_key_2 = v_newStrainKey
WHERE _Strain_key_2 = v_oldStrainKey
;

-- NOTES

FOR v_noteTypeKey IN
SELECT _NoteType_key
FROM MGI_NoteType_Strain_View
LOOP
    -- if both old and new strains have notes, concatenate old notes onto new notes

    IF EXISTS (select 1 from MGI_Note 
		WHERE _MGIType_key = 10 and _NoteType_key = v_noteTypeKey and _Object_key = v_newStrainKey)
       AND
       EXISTS (select 1 from MGI_Note 
		WHERE _MGIType_key = 10 and _NoteType_key = v_noteTypeKey and _Object_key = v_oldStrainKey)
    THEN

    	v_note1 := n.note from MGI_Note n WHERE n._MGIType_key = 10 and n._NoteType_key = v_noteTypeKey and n._Object_key = v_newStrainKey;

	v_note2 := n.note from MGI_Note n WHERE n._MGIType_key = 10 and n._NoteType_key = v_noteTypeKey and n._Object_key = v_oldStrainKey;

        IF v_note1 != v_note2 
        THEN
                v_note1 = v_note1 || E'\n' || v_note2;
        END IF;

	UPDATE MGI_Note
	SET note = v_note1
	WHERE _MGIType_key = 10 
	      AND _NoteType_key = v_noteTypeKey 
	      AND _Object_key = v_newStrainKey 
	;

	DELETE FROM MGI_Note WHERE _MGIType_key = 10 and _NoteType_key = v_noteTypeKey and _Object_key = v_oldStrainKey;

    -- else if only old strain has notes, move old notes to new notes 

    ELSIF NOT EXISTS (SELECT 1 FROM MGI_Note 
		WHERE _MGIType_key = 10 and _NoteType_key = v_noteTypeKey and _Object_key = v_newStrainKey)
       AND
       EXISTS (SELECT 1 FROM MGI_Note 
		WHERE _MGIType_key = 10 and _NoteType_key = v_noteTypeKey and _Object_key = v_oldStrainKey)
    THEN
        UPDATE MGI_Note 
        SET _Object_key = v_newStrainKey 
        WHERE _MGIType_key = 10 and _NoteType_key = v_noteTypeKey and _Object_key = v_oldStrainKey
	;
    END IF;

    -- else if only new strain has notes, do nothing 

END LOOP;

-- END NOTES

-- STRAIN/MARKER/ALLELES

-- remove duplicates
DELETE FROM PRB_Strain_Marker p1
USING PRB_Strain_Marker p2
WHERE p1._Strain_key = v_oldStrainKey
AND p1._Marker_key = p2._Marker_key
AND p1._Allele_key = p2._Allele_key
AND p2._Strain_key = v_newStrainKey
;

UPDATE PRB_Strain_Marker
SET _Strain_key = v_newStrainKey
WHERE _Strain_key = v_oldStrainKey
;

-- TRANSLATIONS

-- remove duplicates 
DELETE FROM MGI_Translation t1
USING MGI_TranslationType tt, MGI_Translation t2
WHERE tt._MGIType_key = 10
and tt._TranslationType_key = t1._TranslationType_key
and t1._Object_key = v_oldStrainKey
and tt._TranslationType_key = t2._TranslationType_key
and t2._Object_key = v_newStrainKey
and t1.badName = t2.badName
;

UPDATE MGI_Translation
SET _Object_key = v_newStrainKey
FROM MGI_TranslationType tt
WHERE tt._MGIType_key = 10
AND tt._TranslationType_key = MGI_Translation._TranslationType_key
AND MGI_Translation._Object_key = v_oldStrainKey
;

-- SETS 

-- remove duplicates 
DELETE FROM MGI_SetMember s1
USING MGI_Set s, MGI_SetMember s2
WHERE s._MGIType_key = 10
AND s._Set_key = s1._Set_key
AND s1._Object_key = v_oldStrainKey
AND s._Set_key = s2._Set_key
AND s2._Object_key = v_newStrainKey
;

UPDATE MGI_SetMember
SET _Object_key = v_newStrainKey
FROM MGI_Set s
WHERE s._MGIType_key = 10
and s._Set_key = MGI_SetMember._Set_key
and MGI_SetMember._Object_key = v_oldStrainKey
;

-- SYNONYMS 

UPDATE MGI_Synonym
SET _Object_key = v_newStrainKey
WHERE _Object_key = v_oldStrainKey
AND _MGIType_key = 10
;

-- if the strain names are not equal 
--   a.  make old strain name a synonym of the new strain 
--   b.  make old strain name a translation of the new strain 

IF (SELECT strain FROM PRB_Strain WHERE _Strain_key = v_oldStrainKey) !=
   (SELECT strain FROM PRB_Strain WHERE _Strain_key = v_newStrainKey)
THEN
	v_nextKey := nextval('mgi_synonym_seq');
	--v_synTypeKey := _SynonymType_key from MGI_SynonymType_Strain_View  WHERE synonymType = 'nomenclature history';
	v_synTypeKey := 1001;

	IF v_nextKey IS NULL 
	THEN
		v_nextKey := 1000;
	END IF;

	INSERT INTO MGI_Synonym (_Synonym_key, _MGIType_key, _Object_key, _SynonymType_key, _Refs_key, synonym)
	SELECT v_nextKey, 10, v_newStrainKey, v_synTypeKey, null, strain
	FROM PRB_Strain WHERE _Strain_key = v_oldStrainKey
	;

	v_nextKey := max(_Translation_key) + 1 from MGI_Translation;
	v_nextSeqKey := max(sequenceNum) + 1 from MGI_Translation WHERE _TranslationType_key = 1007;

	INSERT INTO MGI_Translation (_Translation_key, _TranslationType_key, _Object_key, badName, sequenceNum,
		_CreatedBy_key, _ModifiedBy_key, creation_date, modification_date)
	SELECT v_nextKey, 1007, v_newStrainKey, strain, v_nextSeqKey, v_strainmergeKey, v_strainmergeKey,
		now(), now()
	FROM PRB_Strain WHERE _Strain_key = v_oldStrainKey
	;
END IF;

DELETE FROM PRB_Strain WHERE _Strain_key = v_oldStrainKey;

END;
$$;


--
-- Name: prb_probe_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.prb_probe_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: PRB_Probe_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: prb_probe_insert(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.prb_probe_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: PRB_Probe_insert()
--
-- DESCRIPTOIN:
--
--	this insert trigger will call ACC_assignMGI
--	in order to add a distinct MGI accession id
--	to the NEW object
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

PERFORM ACC_assignMGI(1001, NEW._Probe_key, 'Segment');

RETURN NEW;

END;
$$;


--
-- Name: prb_processanonymoussource(integer, integer, integer, integer, integer, integer, integer, integer, text, text, integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

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


--
-- Name: prb_processprobesource(integer, integer, integer, integer, integer, integer, integer, integer, text, text, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

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


--
-- Name: prb_processseqloadersource(integer, integer, integer, integer, integer, integer, integer, integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.prb_processseqloadersource(v_assockey integer, v_objectkey integer, v_msokey integer, v_organismkey integer, v_strainkey integer, v_tissuekey integer, v_genderkey integer, v_celllinekey integer, v_modifiedbykey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_isAnon int;
v_newMSOKey int;

BEGIN

--
-- process (modify) a Sequence's Molecular Source       
-- from the Sequence Loader                             
--
-- if changing old Named -> new Named, then v_msoKey = _Source_key of new Named source 
-- if changing old Anon -> new Named, then v_msoKey = _Source_key of new Named source 
--
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


--
-- Name: prb_processsequencesource(integer, integer, integer, integer, integer, integer, integer, integer, integer, text, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.prb_processsequencesource(v_isanon integer, v_assockey integer, v_objectkey integer, v_msokey integer, v_organismkey integer, v_strainkey integer, v_tissuekey integer, v_genderkey integer, v_celllinekey integer, v_age text, v_modifiedbykey integer, OUT v_newmsokey integer) RETURNS integer
    LANGUAGE plpgsql
    AS $$

DECLARE
v_newAssocKey int;
v_segmentTypeKey int;
v_vectorTypeKey int;

BEGIN

--
-- process (modify) a Sequence's Molecular Source  
-- 
-- if changing old Named -> new Named, then v_msoKey = _Source_key of new Named source 
-- if changing old Anon -> new Named, then v_msoKey = _Source_key of new Named source 
--
-- if changing old Named -> Anonymous, then v_msoKey = _Source_key of old Named source
-- if changing old Anon -> new Anon, then v_msoKey = _Source_key of old Anon source
--

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


--
-- Name: prb_reference_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.prb_reference_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: PRB_Reference_delete()
--
-- DESCRIPTOIN:
--	
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: prb_reference_update(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.prb_reference_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: PRB_Reference_update()
--
-- DESCRIPTOIN:
--
-- update the accession reference that is attached to this J:
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: prb_setstrainreview(integer, integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.prb_setstrainreview(v_markerkey integer DEFAULT NULL::integer, v_allelekey integer DEFAULT NULL::integer, v_reviewkey integer DEFAULT 8581446) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_annotTypeKey int;
v_qualifierKey int;
v_strainKey int;

BEGIN

--
-- sets the "Needs Review" flag for all strains of a marker or allele.
--
-- default "Needs Review - symbol" flag for all strains of a marker or allele (8581446).
--
-- called during nomenclature update process (MRK_simpleWithdrawal, MRK_updateKeys)
-- or when allele symbols is updated (trigger/ALL_Allele)
-- or when a phenotype mutant is withdrawn as an allele of (MRK_mergeWithdrawal)
--

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


--
-- Name: prb_source_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.prb_source_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: PRB_Source_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: prb_strain_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.prb_strain_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: PRB_Strain_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: prb_strain_insert(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.prb_strain_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: PRB_Strain_insert()
--
-- DESCRIPTOIN:
--
--	this insert trigger will call ACC_assignMGI
--	in order to add a distinct MGI accession id
--	to the NEW object
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

PERFORM ACC_assignMGI(1001, NEW._Strain_key, 'Strain');

RETURN NEW;

END;
$$;


--
-- Name: seq_deletebycreatedby(text); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.seq_deletebycreatedby(v_createdby text) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_userKey int;

BEGIN

--
-- NAME: SEQ_deleteByCreatedBy
--
-- DESCRIPTION:
--
-- Delete Sequence objects by Created By
--
-- INPUT:
--      
-- v_createdBy mgi_user.login%TYPE
--
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


--
-- Name: seq_deletedummy(integer); Type: FUNCTION; Schema: mgd; Owner: -
--

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


--
-- Name: seq_deleteobsoletedummy(); Type: FUNCTION; Schema: mgd; Owner: -
--

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


--
-- Name: seq_merge(text, text); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.seq_merge(v_fromseqid text, v_toseqid text) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_fromSeqKey int;
v_toSeqKey int;

BEGIN

--
-- Merge v_fromSeqID to v_toSeqID
--
-- 1) Move non-duplicate Seq ID/Reference associations (MGI_Reference)
-- 2) Make non-duplicate "from" Seq IDs secondary to the "to" Sequence object
-- 3) Delete the "from" Sequence object
--
--

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


--
-- Name: seq_sequence_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.seq_sequence_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: SEQ_Sequence_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: seq_split(text, text); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.seq_split(v_fromseqid text, v_toseqids text) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_fromSeqKey int;
v_splitStatusKey int;
v_toSeqKey int;
v_toAccID acc_accession.accID%TYPE;
v_idx text;
v_accID acc_accession.accID%TYPE;
v_logicalDB int;

BEGIN

--
-- Split v_fromSeqID to v_toSeqIDs
-- where v_toSeqIDs is a comma-separated list of Seq IDs to split the v_fromSeqID into
--
-- 1) Copy non-duplicate "from" Accession IDs to each "to" Sequence object and make them Secondary IDs
-- 2) Status the "from" Sequence object as "Split"
--
--

v_fromSeqKey := _Object_key from SEQ_Sequence_Acc_View where accID = v_fromSeqID and preferred = 1;
v_splitStatusKey := _Term_key from VOC_Term where _Vocab_key = 20 and term = 'SPLIT';

IF v_fromSeqKey IS NULL
THEN
	RAISE EXCEPTION 'SEQ_split: Could not resolve %.', v_fromSeqID;
	RETURN;
END IF;

-- delete cache entries from old sequence

DELETE FROM SEQ_Marker_Cache where _Sequence_key = v_fromSeqKey;
DELETE FROM SEQ_Probe_Cache where _Sequence_key = v_fromSeqKey;

-- For each new Sequence:
--
--      1. copy non-duplicate "from" Accession IDs to each "to" Sequence object and make them Secondary IDs
--      2. re-load SEQ_Marker_Cache
--      3. re-load SEQ_Probe_Cache

WHILE (v_toSeqIDs != NULL) LOOP
	v_idx := position(',' in v_toSeqIDs);

	IF v_idx > 0
	THEN
		v_toAccID := substring(v_toSeqIDs, 1, v_idx - 1);
		v_toSeqIDs := substring(v_toSeqIDs, v_idx + 1, char_length(v_toSeqIDs));
	ELSE
		-- at end of list of v_toSeqIDs
		v_toAccID := v_toSeqIDs;
		v_toSeqIDs := NULL;
	END IF;

	IF v_toAccID != NULL
	THEN
		v_toSeqKey := _Object_key from SEQ_Sequence_Acc_View where accID = v_toAccID and preferred = 1;

		IF v_toSeqKey is null
		THEN
			RAISE EXCEPTION E'SEQ_split: Could not resolve %.', v_toAccID;
			RETURN;
		END IF;

                -- delete cache entries from new sequence

                DELETE FROM SEQ_Marker_Cache where _Sequence_key = v_toSeqKey;
                DELETE FROM SEQ_Probe_Cache where _Sequence_key = v_toSeqKey;

		-- copy all Accession IDs of v_fromSeqKey to v_toSeqKey - make them all secondary
		FOR v_accID, v_logicalDB IN
		SELECT a1.accID, a1._LogicalDB_key
		FROM ACC_Accession a1
		WHERE a1._MGIType_key = 19
		AND a1._Object_key = v_fromSeqKey
		AND NOT EXISTS (SELECT 1 FROM ACC_Accession a2
			WHERE a2._MGIType_key = 19
			AND a2._Object_key = v_toSeqKey
			AND a2.accID = a1.accID)
		LOOP
			PERFORM ACC_insert (1001, v_toSeqKey, v_accID, v_logicalDB, 'Sequence', -1, 0, 0);
		END LOOP;
	END IF;
END LOOP;

-- Now make the final necessary modifications to the old Sequence object */

UPDATE SEQ_Sequence
SET _SequenceStatus_key = v_splitStatusKey, _ModifiedBy_key = 1001, modification_date = now()
WHERE _Sequence_key = v_fromSeqKey
;

END;
$$;


--
-- Name: voc_annot_insert(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.voc_annot_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$

BEGIN

--
-- NAME: VOC_Annot_insert()
--
-- DESCRIPTOIN:
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

IF (SELECT t.isObsolete FROM VOC_Term t WHERE t._Term_key = NEW._Term_key) = 1
THEN
	RAISE EXCEPTION E'Cannot Annotate to an Obsolete Term.';
END IF;

RETURN NEW;

END;
$$;


--
-- Name: voc_copyannotevidencenotes(integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.voc_copyannotevidencenotes(v_userkey integer, v_fromannotevidencekey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_noteKey int;

BEGIN

--
-- NAME: VOC_copyAnnotEvidenceNotes
--
-- DESCRIPTION:
--        
-- copy Annotation Evidence Notes from one Evidence record to another
--
-- INPUT:
--      
-- v_userKey 	          : MGI_User._User_key
-- v_fromAnnotEvidenceKey : VOC_Evidence._AnnotEvidence_key
--
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


--
-- Name: voc_evidence_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.voc_evidence_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: VOC_Evidence_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


--
-- Name: voc_evidence_insert(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.voc_evidence_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$

DECLARE
v_termKey int;
v_objectKey int;
v_dagKey int;

BEGIN

--
-- NAME: VOC_Evidence_insert()
--
-- DESCRIPTOIN:
--
--	1) if AP/GO annotation 
--              if Reference exists as group = 'AP', 'GO'
--		in BIB_Workflow_Status,
--	   then
--		set existing BIB_Workflow_Status.isCurrent = 0
--		add new BIB_Workflow_Status._Status_key = 'Full-coded'
--
-- changes to this trigger require changes to procedure/BIB_updateWFStatusAP_create.object
-- changes to this trigger require changes to procedure/BIB_updateWFStatusGO_create.object
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

v_termKey := _Term_key FROM VOC_Annot a WHERE a._Annot_key = NEW._Annot_key;

v_objectKey := _Object_key FROM VOC_Annot a WHERE a._Annot_key = NEW._Annot_key;

v_dagKey := distinct _DAG_key FROM VOC_Annot_View a, DAG_Node_View d
        WHERE a._Annot_key = NEW._Annot_key
        AND a._Vocab_key = d._Vocab_key
        AND a._Term_key = d._Object_key;

--
-- TR 7865: Disallow Annotation if adding an Annotation to an "unknown" term (120, 1098, 6113)
-- and an Annotation to a known Term in the same DAG already exists
-- If the Annotation to a known Term is to
-- J:72247 (InterPro)
-- J:60000 (Swiss-Prot)
-- J:72245 (Swiss-Prot)
-- J:80000 (RIKEN)
-- J:56000
-- then it's okay 
--

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


--
-- Name: voc_evidence_property_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.voc_evidence_property_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: VOC_Evidence_Property_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

DELETE FROM MGI_Note a
WHERE a._Object_key = OLD._EvidenceProperty_key
AND a._MGIType_key = 41
;


RETURN NEW;

END;
$$;


--
-- Name: voc_evidence_update(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.voc_evidence_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: VOC_Evidence_update()
--
-- DESCRIPTOIN:
--	
-- after update, if there are no more Evidence records to the Annotation, 
--	then delete the Annotation record as well
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

IF NOT EXISTS (SELECT 1 FROM VOC_Evidence where OLD._Annot_key = VOC_Evidence._Annot_key)
THEN
        DELETE FROM VOC_Annot a
	WHERE a._Annot_key = OLD._Annot_key
	;
END IF;

RETURN NEW;

END;
$$;


--
-- Name: voc_mergeannotations(integer, integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.voc_mergeannotations(annottypekey integer, oldkey integer, newkey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

BEGIN

--
-- NAME: VOC_mergeAnnotations
--
-- DESCRIPTION:
--        
-- Executed from MRK_updateKeys during merge process 
--
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
--
-- INPUT:
--      
-- annotTypeKey : VOC_Annot._AnnotType_key
-- oldKey       : VOC_Annot._Annot_key "from" key
-- newKey       : VOC_Annot._Annot_key "to" key
--
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


--
-- Name: voc_mergedupannotations(integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

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

--
-- NAME: VOC_mergeDupAnnotations
--
-- DESCRIPTION:
--        
-- Executed from API/AnotationService/process()
--
-- A unique VOC_Annotation record is defined by: 
--	_AnnotType_key	
--	_Object_key	
--	_Term_key	
--	_Qualifier_key	
--
-- For given _Object_key, find any duplicate VOC_Annot rows and:
--
-- move VOC_Evidence._Annot_key from bad/duplidate VOC_Annot record to "good" VOC_Annot record
-- delete duplicate VOC_Annot record
--
-- INPUT:
--      
-- annotTypeKey : VOC_Annot._AnnotType_key
-- in_objectKey    : VOC_Annot._Object_key
--
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


--
-- Name: voc_mergeterms(integer, integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.voc_mergeterms(oldkey integer, newkey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

BEGIN

--
-- NAME: VOC_mergeTerms
--
-- DESCRIPTION:
--        
-- To merge a term:
--
-- a.  make the accID of the old term a secondary accID of the new term 
-- b.  move annotations of the old term to the new term (avoid duplicates)
-- c.  delete the old term                                               
--
-- a.  make the accID of the old term a secondary accID of the new term 
--
-- INPUT:
--      
-- oldKey : VOC_Term._Term_key : old
-- newKey : VOC_Term._Term_key : new
--
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


--
-- Name: voc_processannotheader(integer, integer, integer, smallint); Type: FUNCTION; Schema: mgd; Owner: -
--

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

--
-- NAME: VOC_processAnnotHeader
--
-- DESCRIPTION:
--        
-- set headers by Annotation Type, Object
--
-- INPUT:
--      
-- v_userKey      : MGI_User._User_key
-- v_annotTypeKey : VOC_Annot._AnnotType_key
-- v_objectKey    : VOC_Annot._Object_key
-- v_reorder      : DEFAULT 1, if updating the cache for a single object, 
--	            then re-set the sequence numbers so there are no gaps
--
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


--
-- Name: voc_processannotheaderall(integer); Type: FUNCTION; Schema: mgd; Owner: -
--

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

--
-- NAME: VOC_processAnnotHeaderAll
--
-- DESCRIPTION:
--        
-- incrementally update VOC_AnnotHeader by annotation type
--
-- INPUT:
--      
-- v_annotTypeKey : VOC_Annot._AnnotType_key
--
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


--
-- Name: voc_processannotheadermissing(integer); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.voc_processannotheadermissing(v_annottypekey integer) RETURNS void
    LANGUAGE plpgsql
    AS $$

DECLARE
v_objectKey int;

BEGIN

--
-- NAME: VOC_processAnnotHeaderMissing
--
-- DESCRIPTION:
--        
-- add missing VOC_AnnotHeader records by annotation type
--
-- INPUT:
--      
-- v_annotTypeKey : VOC_Annot._AnnotType_key
--
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


--
-- Name: voc_resetterms(integer); Type: FUNCTION; Schema: mgd; Owner: -
--

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


--
-- Name: voc_term_delete(); Type: FUNCTION; Schema: mgd; Owner: -
--

CREATE FUNCTION mgd.voc_term_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN

--
-- NAME: VOC_Term_delete()
--
-- DESCRIPTOIN:
--	
-- the template creates a trigger for all tables that use:
--	_Object_key
--	_MGIType_key
--
-- because these tables cannot use referencial integrity due
-- to the fact that their primary key is not necessarily the
-- member of the _Object_key/_MGIType_key table
--
-- . remove the statements that are not active for your table
-- . add the statements that are being active for your table
--
-- or you may keep all statements in the trigger even if your table
-- does not currently utilize them.
--
-- An example of a table that only needs ACC_Accession : MGI_Organism
--
-- INPUT:
--	none
--
-- RETURNS:
--	NEW
--

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


SET default_table_access_method = heap;

--
-- Name: acc_accession; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: acc_accessionmax; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.acc_accessionmax (
    prefixpart text NOT NULL,
    maxnumericpart integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: acc_accessionreference; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.acc_accessionreference (
    _accession_key integer NOT NULL,
    _refs_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: acc_actualdb_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.acc_actualdb_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: acc_actualdb; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: acc_logicaldb_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.acc_logicaldb_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: acc_logicaldb; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: acc_mgitype; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: acc_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: all_acc_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: all_allele_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.all_allele_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: all_allele; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: all_allele_cellline_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.all_allele_cellline_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: all_allele_cellline; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.all_allele_cellline (
    _assoc_key integer DEFAULT nextval('mgd.all_allele_cellline_seq'::regclass) NOT NULL,
    _allele_key integer NOT NULL,
    _mutantcellline_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: all_cellline_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.all_cellline_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: all_cellline; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: all_cellline_derivation_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.all_cellline_derivation_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: all_cellline_derivation; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_user; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: prb_strain_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.prb_strain_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: prb_strain; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: voc_term_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.voc_term_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: voc_term; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: all_allele_cellline_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_organism_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.mgi_organism_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mgi_organism; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.mgi_organism (
    _organism_key integer DEFAULT nextval('mgd.mgi_organism_seq'::regclass) NOT NULL,
    commonname text NOT NULL,
    latinname text,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: mgi_relationship_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.mgi_relationship_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mgi_relationship; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mrk_marker_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.mrk_marker_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mrk_marker; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: all_allele_driver_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: all_allele_mutation_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.all_allele_mutation_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: all_allele_mutation; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.all_allele_mutation (
    _assoc_key integer DEFAULT nextval('mgd.all_allele_mutation_seq'::regclass) NOT NULL,
    _allele_key integer NOT NULL,
    _mutation_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: all_allele_mutation_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: voc_annot_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.voc_annot_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: voc_annot; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.voc_annot (
    _annot_key integer DEFAULT nextval('mgd.voc_annot_seq'::regclass) NOT NULL,
    _annottype_key integer NOT NULL,
    _object_key integer NOT NULL,
    _term_key integer NOT NULL,
    _qualifier_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: all_allele_subtype_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: bib_citation_cache; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: all_allele_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_allelepair_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_allelepair_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_allelepair; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: all_genotype_view; Type: VIEW; Schema: mgd; Owner: -
--

CREATE VIEW mgd.all_genotype_view AS
 SELECT DISTINCT gxd_allelepair._genotype_key,
    gxd_allelepair._allele_key_1 AS _allele_key
   FROM mgd.gxd_allelepair
UNION
 SELECT DISTINCT gxd_allelepair._genotype_key,
    gxd_allelepair._allele_key_2 AS _allele_key
   FROM mgd.gxd_allelepair
  WHERE (gxd_allelepair._allele_key_2 IS NOT NULL);


--
-- Name: voc_annottype; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: all_annot_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: all_cellline_acc_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: all_cellline_derivation_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: all_cellline_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: all_cre_cache; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: all_knockout_cache; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: all_label; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: all_summary_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_allelegenotype; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: all_summarybymarker_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_reference_assoc_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.mgi_reference_assoc_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mgi_reference_assoc; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: all_summarybyreference_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: all_variant_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.all_variant_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: all_variant; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: all_variantsequence_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.all_variantsequence_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: all_variant_sequence; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: bib_acc_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: bib_refs_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.bib_refs_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: bib_refs; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: bib_all_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_expression; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_index_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_index_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_index; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: img_image_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.img_image_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: img_image; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_refassoctype; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_reference_strain_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_synonym_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.mgi_synonym_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mgi_synonym; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_synonymtype; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_synonym_strain_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mld_expts_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.mld_expts_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mld_expts; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.mld_expts (
    _expt_key integer DEFAULT nextval('mgd.mld_expts_seq'::regclass) NOT NULL,
    _refs_key integer NOT NULL,
    expttype text NOT NULL,
    tag integer NOT NULL,
    chromosome text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: mld_notes; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.mld_notes (
    _refs_key integer NOT NULL,
    note text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: mrk_do_cache; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mrk_reference; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mrk_strainmarker_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.mrk_strainmarker_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mrk_strainmarker; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: prb_reference_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.prb_reference_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: prb_reference; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: prb_source_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.prb_source_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: prb_source; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: voc_evidence_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.voc_evidence_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: voc_evidence; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: bib_associateddata_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: bib_books; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: bib_workflow_status_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.bib_workflow_status_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: bib_workflow_status; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: bib_goxref_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: bib_notes; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.bib_notes (
    _refs_key integer NOT NULL,
    note text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: bib_status_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_assay_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_assay_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_assay; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_specimen_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_specimen_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_specimen; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: bib_summary_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: bib_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: bib_workflow_data_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.bib_workflow_data_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: bib_workflow_data; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: bib_workflow_relevance_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.bib_workflow_relevance_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: bib_workflow_relevance; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: bib_workflow_tag_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.bib_workflow_tag_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: bib_workflow_tag; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.bib_workflow_tag (
    _assoc_key integer DEFAULT nextval('mgd.bib_workflow_tag_seq'::regclass) NOT NULL,
    _refs_key integer NOT NULL,
    _tag_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: crs_cross; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: crs_matrix; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: crs_progeny; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.crs_progeny (
    _cross_key integer NOT NULL,
    sequencenum integer NOT NULL,
    name text,
    sex character(1) NOT NULL,
    notes text,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: crs_references; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.crs_references (
    _cross_key integer NOT NULL,
    _marker_key integer NOT NULL,
    _refs_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: crs_typings; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.crs_typings (
    _cross_key integer NOT NULL,
    rownumber integer NOT NULL,
    colnumber integer NOT NULL,
    data text,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: dag_closure; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: dag_dag; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.dag_dag (
    _dag_key integer NOT NULL,
    _refs_key integer NOT NULL,
    _mgitype_key integer NOT NULL,
    name text NOT NULL,
    abbreviation character(5) NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: dag_edge; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: dag_label; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.dag_label (
    _label_key integer NOT NULL,
    label text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: dag_node; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.dag_node (
    _node_key integer NOT NULL,
    _dag_key integer NOT NULL,
    _object_key integer NOT NULL,
    _label_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: voc_vocabdag; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.voc_vocabdag (
    _vocab_key integer NOT NULL,
    _dag_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: dag_node_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: go_tracking; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_allelepair_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_antibody_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_antibody_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_antibody; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_antibody_acc_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_antigen_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_antigen_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_antigen; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_antibody_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_antibodyalias_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_antibodyalias_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_antibodyalias; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.gxd_antibodyalias (
    _antibodyalias_key integer DEFAULT nextval('mgd.gxd_antibodyalias_seq'::regclass) NOT NULL,
    _antibody_key integer NOT NULL,
    _refs_key integer,
    alias text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: gxd_antibodyalias_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_antibodyaliasref_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_antigen_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_antibodyantigen_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_antibodymarker_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_antibodymarker_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_antibodymarker; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.gxd_antibodymarker (
    _antibodymarker_key integer DEFAULT nextval('mgd.gxd_antibodymarker_seq'::regclass) NOT NULL,
    _antibody_key integer NOT NULL,
    _marker_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: gxd_antibodymarker_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_antibodyprep_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_antibodyprep_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_antibodyprep; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.gxd_antibodyprep (
    _antibodyprep_key integer DEFAULT nextval('mgd.gxd_antibodyprep_seq'::regclass) NOT NULL,
    _antibody_key integer NOT NULL,
    _secondary_key integer NOT NULL,
    _label_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: gxd_antibodyprep_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_antigen_acc_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_antigen_summary_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_assay_acc_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_gellane_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_gellane_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_gellane; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_assay_allele_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_insituresult_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_insituresult_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_insituresult; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_insituresultimage_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_insituresultimage_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_insituresultimage; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.gxd_insituresultimage (
    _resultimage_key integer DEFAULT nextval('mgd.gxd_insituresultimage_seq'::regclass) NOT NULL,
    _result_key integer NOT NULL,
    _imagepane_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: gxd_assay_dltemplate_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_assaytype_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_assaytype_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_assaytype; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.gxd_assaytype (
    _assaytype_key integer DEFAULT nextval('mgd.gxd_assaytype_seq'::regclass) NOT NULL,
    assaytype text NOT NULL,
    isrnaassay smallint NOT NULL,
    isgelassay smallint NOT NULL,
    sequencenum integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: gxd_assay_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_assaynote_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_assaynote_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_assaynote; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.gxd_assaynote (
    _assaynote_key integer DEFAULT nextval('mgd.gxd_assaynote_seq'::regclass) NOT NULL,
    _assay_key integer NOT NULL,
    assaynote text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: gxd_gelband_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_gelband_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_gelband; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.gxd_gelband (
    _gelband_key integer DEFAULT nextval('mgd.gxd_gelband_seq'::regclass) NOT NULL,
    _gellane_key integer NOT NULL,
    _gelrow_key integer NOT NULL,
    _strength_key integer NOT NULL,
    bandnote text,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: gxd_gelrow_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_gelrow_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_gelrow; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_gelband_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_genotype_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_genotype_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_genotype; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_gellane_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_gellanestructure_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_gellanestructure_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_gellanestructure; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.gxd_gellanestructure (
    _gellanestructure_key integer DEFAULT nextval('mgd.gxd_gellanestructure_seq'::regclass) NOT NULL,
    _gellane_key integer NOT NULL,
    _emapa_term_key integer NOT NULL,
    _stage_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: gxd_theilerstage; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.gxd_theilerstage (
    _stage_key integer NOT NULL,
    stage integer NOT NULL,
    description text,
    dpcmin numeric NOT NULL,
    dpcmax numeric NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: gxd_gellanestructure_view; Type: VIEW; Schema: mgd; Owner: -
--

CREATE VIEW mgd.gxd_gellanestructure_view AS
 SELECT l._assay_key,
    l.sequencenum,
    g._gellanestructure_key,
    g._gellane_key,
    g._emapa_term_key,
    g._stage_key,
    g.creation_date,
    g.modification_date,
    s.term,
    t.stage,
    ((('TS'::text || ((t.stage)::character varying(5))::text) || ';'::text) || s.term) AS displayit
   FROM mgd.gxd_gellane l,
    mgd.gxd_gellanestructure g,
    mgd.voc_term s,
    mgd.gxd_theilerstage t
  WHERE ((l._gellane_key = g._gellane_key) AND (g._emapa_term_key = s._term_key) AND (g._stage_key = t._stage_key));


--
-- Name: gxd_gelrow_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_genotype_acc_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_genotype_dataset_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_genotype_summary_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_genotype_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: voc_annotheader_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.voc_annotheader_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: voc_annotheader; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_genotypeannotheader_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_htexperiment_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_htexperiment_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_htexperiment; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_htexperimentvariable_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_htexperimentvariable_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_htexperimentvariable; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.gxd_htexperimentvariable (
    _experimentvariable_key integer DEFAULT nextval('mgd.gxd_htexperimentvariable_seq'::regclass) NOT NULL,
    _experiment_key integer NOT NULL,
    _term_key integer NOT NULL
);


--
-- Name: gxd_htrawsample_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_htrawsample_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_htrawsample; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.gxd_htrawsample (
    _rawsample_key integer DEFAULT nextval('mgd.gxd_htrawsample_seq'::regclass) NOT NULL,
    _experiment_key integer NOT NULL,
    accid text,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: gxd_htsample_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_htsample_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_htsample; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_htsample_rnaseq_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_htsample_rnaseq_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_htsample_rnaseq; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_htsample_rnaseqcombined_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_htsample_rnaseqcombined_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_htsample_rnaseqcombined; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_htsample_rnaseqset_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_htsample_rnaseqset_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_htsample_rnaseqset; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_htsample_rnaseqset_cache_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_htsample_rnaseqset_cache_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_htsample_rnaseqset_cache; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.gxd_htsample_rnaseqset_cache (
    _assoc_key integer DEFAULT nextval('mgd.gxd_htsample_rnaseqset_cache_seq'::regclass) NOT NULL,
    _rnaseqcombined_key integer NOT NULL,
    _rnaseqset_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: gxd_htsample_rnaseqsetmember_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_htsample_rnaseqsetmember_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_htsample_rnaseqsetmember; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.gxd_htsample_rnaseqsetmember (
    _rnaseqsetmember_key integer DEFAULT nextval('mgd.gxd_htsample_rnaseqsetmember_seq'::regclass) NOT NULL,
    _rnaseqset_key integer NOT NULL,
    _sample_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: gxd_indexstage_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_indexstage_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_index_stages; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_index_summaryby_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_index_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_insituresult_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_isresultcelltype_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_isresultcelltype_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_isresultcelltype; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.gxd_isresultcelltype (
    _resultcelltype_key integer DEFAULT nextval('mgd.gxd_isresultcelltype_seq'::regclass) NOT NULL,
    _result_key integer NOT NULL,
    _celltype_term_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: gxd_isresultcelltype_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: img_imagepane_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.img_imagepane_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: img_imagepane; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_isresultimage_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_isresultstructure_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_isresultstructure_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_isresultstructure; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.gxd_isresultstructure (
    _resultstructure_key integer DEFAULT nextval('mgd.gxd_isresultstructure_seq'::regclass) NOT NULL,
    _result_key integer NOT NULL,
    _emapa_term_key integer NOT NULL,
    _stage_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: gxd_isresultstructure_view; Type: VIEW; Schema: mgd; Owner: -
--

CREATE VIEW mgd.gxd_isresultstructure_view AS
 SELECT r._specimen_key,
    r.sequencenum,
    g._resultstructure_key,
    g._result_key,
    g._emapa_term_key,
    g._stage_key,
    g.creation_date,
    g.modification_date,
    s.term,
    t.stage,
    ((('TS'::text || ((t.stage)::character varying(5))::text) || ';'::text) || s.term) AS displayit
   FROM mgd.gxd_insituresult r,
    mgd.gxd_isresultstructure g,
    mgd.voc_term s,
    mgd.gxd_theilerstage t
  WHERE ((r._result_key = g._result_key) AND (g._emapa_term_key = s._term_key) AND (g._stage_key = t._stage_key));


--
-- Name: gxd_probeprep_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.gxd_probeprep_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gxd_probeprep; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: prb_probe_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.prb_probe_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: prb_probe; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_probeprep_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: gxd_specimen_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: img_image_acc_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: img_image_summary2_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: img_image_summarybyreference_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: img_image_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: img_imagepane_assoc_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.img_imagepane_assoc_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: img_imagepane_assoc; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: img_imagepane_assoc_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: img_imagepaneallele_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_note_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.mgi_note_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mgi_note; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: img_imagepanegenotype_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: img_imagepanegxd_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: map_coord_collection; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.map_coord_collection (
    _collection_key integer NOT NULL,
    name text NOT NULL,
    abbreviation text NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: map_coord_feature; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: map_coordinate; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mrk_chromosome_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.mrk_chromosome_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mrk_chromosome; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: map_gm_coord_feature_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_dbinfo; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_keyvalue_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.mgi_keyvalue_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mgi_keyvalue; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_notetype; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_note_allele_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_note_allelevariant_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_note_derivation_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_note_genotype_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_note_image_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_note_marker_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_note_probe_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_note_strain_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_note_vocevidence_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_notetype_strain_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_organism_mgitype_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.mgi_organism_mgitype_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mgi_organism_mgitype; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_organism_allele_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_organism_antigen_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_organism_marker_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_organism_mgitype_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_organism_probe_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_property_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.mgi_property_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mgi_property; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_propertytype; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_reference_allele_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_reference_allelevariant_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_reference_antibody_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_reference_doid_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_reference_marker_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_relationship_category; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_relationship_fear_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_relationship_markerqtlcandidate_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_relationship_markerqtlinteraction_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_relationship_markertss_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_relationship_property_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.mgi_relationship_property_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mgi_relationship_property; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_set; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_setmember; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_setmember_emapa; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.mgi_setmember_emapa (
    _setmember_emapa_key integer NOT NULL,
    _setmember_key integer NOT NULL,
    _stage_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: mgi_synonym_allele_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_synonym_musmarker_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_synonymtype_strain_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_translation_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.mgi_translation_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mgi_translation; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_translationtype; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: voc_vocab; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mgi_user_active_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mld_assay_types_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.mld_assay_types_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mld_assay_types; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.mld_assay_types (
    _assay_type_key integer DEFAULT nextval('mgd.mld_assay_types_seq'::regclass) NOT NULL,
    description text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: mld_concordance; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mld_contig; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mld_contigprobe; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.mld_contigprobe (
    _contig_key integer NOT NULL,
    sequencenum integer NOT NULL,
    _probe_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: mld_expt_marker_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.mld_expt_marker_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mld_expt_marker; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mld_expt_marker_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mld_expt_notes; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.mld_expt_notes (
    _expt_key integer NOT NULL,
    note text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: mld_expt_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mld_fish; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mld_fish_region; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.mld_fish_region (
    _expt_key integer NOT NULL,
    sequencenum integer NOT NULL,
    region text,
    totalsingle integer,
    totaldouble integer,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: mld_hit; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.mld_hit (
    _expt_key integer NOT NULL,
    _probe_key integer NOT NULL,
    _target_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: mld_hybrid; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.mld_hybrid (
    _expt_key integer NOT NULL,
    chrsorgenes smallint NOT NULL,
    band text,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: mld_insitu; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mld_isregion; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.mld_isregion (
    _expt_key integer NOT NULL,
    sequencenum integer NOT NULL,
    region text,
    graincount integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: mld_matrix; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mld_mc2point; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mld_mcdatalist; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.mld_mcdatalist (
    _expt_key integer NOT NULL,
    sequencenum integer NOT NULL,
    alleleline text NOT NULL,
    offspringnmbr integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: mld_ri; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.mld_ri (
    _expt_key integer NOT NULL,
    ri_idlist text NOT NULL,
    _riset_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: mld_ri2point; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mld_ridata; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.mld_ridata (
    _expt_key integer NOT NULL,
    _marker_key integer NOT NULL,
    sequencenum integer NOT NULL,
    alleleline text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: mld_statistics; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mrk_acc_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mrk_types; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.mrk_types (
    _marker_type_key integer NOT NULL,
    name text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: mrk_accnoref_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mrk_accref1_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mrk_accref2_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mrk_biotypemapping; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mrk_cluster_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.mrk_cluster_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mrk_cluster; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mrk_clustermember_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.mrk_clustermember_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mrk_clustermember; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.mrk_clustermember (
    _clustermember_key integer DEFAULT nextval('mgd.mrk_clustermember_seq'::regclass) NOT NULL,
    _cluster_key integer NOT NULL,
    _marker_key integer NOT NULL,
    sequencenum integer NOT NULL
);


--
-- Name: mrk_current; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.mrk_current (
    _current_key integer NOT NULL,
    _marker_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: mrk_current_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mrk_history_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.mrk_history_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mrk_history; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mrk_history_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mrk_label; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mrk_location_cache; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mrk_status; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.mrk_status (
    _marker_status_key integer NOT NULL,
    status text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: mrk_marker_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mrk_mcv_cache; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: mrk_mcv_count_cache; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.mrk_mcv_count_cache (
    _mcvterm_key integer NOT NULL,
    markercount integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: mrk_mouse_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mrk_notes; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.mrk_notes (
    _marker_key integer NOT NULL,
    note text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: mrk_summary_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: mrk_summarybyreference_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: prb_acc_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: prb_accref_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: prb_alias_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.prb_alias_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: prb_alias; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.prb_alias (
    _alias_key integer DEFAULT nextval('mgd.prb_alias_seq'::regclass) NOT NULL,
    _reference_key integer NOT NULL,
    alias text NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: prb_allele_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.prb_allele_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: prb_allele; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: prb_allele_strain_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.prb_allele_strain_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: prb_allele_strain; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.prb_allele_strain (
    _allelestrain_key integer DEFAULT nextval('mgd.prb_allele_strain_seq'::regclass) NOT NULL,
    _allele_key integer NOT NULL,
    _strain_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: prb_marker_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.prb_marker_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: prb_marker; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: prb_marker_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: prb_notes_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.prb_notes_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: prb_notes; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.prb_notes (
    _note_key integer DEFAULT nextval('mgd.prb_notes_seq'::regclass) NOT NULL,
    _probe_key integer NOT NULL,
    note text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: prb_probe_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: prb_ref_notes; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.prb_ref_notes (
    _reference_key integer NOT NULL,
    note text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: prb_rflv_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.prb_rflv_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: prb_rflv; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: prb_source_acc_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: prb_tissue_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.prb_tissue_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: prb_tissue; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.prb_tissue (
    _tissue_key integer DEFAULT nextval('mgd.prb_tissue_seq'::regclass) NOT NULL,
    tissue text NOT NULL,
    standard smallint NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: prb_source_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: prb_strain_acc_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: prb_strain_attribute_view; Type: VIEW; Schema: mgd; Owner: -
--

CREATE VIEW mgd.prb_strain_attribute_view AS
 SELECT va._annot_key,
    va._object_key AS _strain_key,
    va._term_key,
    vt.term
   FROM mgd.voc_annot va,
    mgd.voc_annottype vat,
    mgd.voc_term vt
  WHERE ((vat.name = 'Strain/Attributes'::text) AND (vat._annottype_key = va._annottype_key) AND (va._term_key = vt._term_key));


--
-- Name: prb_strain_genotype_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.prb_strain_genotype_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: prb_strain_genotype; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: prb_strain_genotype_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: prb_strain_marker_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.prb_strain_marker_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: prb_strain_marker; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: prb_strain_marker_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: prb_strain_needsreview_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: prb_strain_reference_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: prb_strain_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: pwi_report_id_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.pwi_report_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: pwi_report_label_id_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.pwi_report_label_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ri_riset; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: ri_summary; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.ri_summary (
    _risummary_key integer NOT NULL,
    _riset_key integer NOT NULL,
    _marker_key integer NOT NULL,
    summary text NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: ri_summary_expt_ref; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.ri_summary_expt_ref (
    _risummary_key integer NOT NULL,
    _expt_key integer NOT NULL,
    _refs_key integer NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: seq_allele_assoc; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: seq_coord_cache; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: seq_genemodel; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: seq_genetrap; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: seq_marker_cache; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: seq_probe_cache; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: seq_sequence; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: seq_sequence_acc_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: seq_sequence_assoc; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: seq_sequence_raw; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: seq_source_assoc_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.seq_source_assoc_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: seq_source_assoc; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.seq_source_assoc (
    _assoc_key integer DEFAULT nextval('mgd.seq_source_assoc_seq'::regclass) NOT NULL,
    _sequence_key integer NOT NULL,
    _source_key integer NOT NULL,
    _createdby_key integer DEFAULT 1001 NOT NULL,
    _modifiedby_key integer DEFAULT 1001 NOT NULL,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: seq_summary_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: voc_allele_cache_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.voc_allele_cache_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: voc_allele_cache; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.voc_allele_cache (
    _cache_key integer DEFAULT nextval('mgd.voc_allele_cache_seq'::regclass) NOT NULL,
    _term_key integer NOT NULL,
    _allele_key integer NOT NULL,
    annottype text NOT NULL
);


--
-- Name: voc_allele_cache__cache_key_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.voc_allele_cache__cache_key_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: voc_allele_cache__cache_key_seq; Type: SEQUENCE OWNED BY; Schema: mgd; Owner: -
--

ALTER SEQUENCE mgd.voc_allele_cache__cache_key_seq OWNED BY mgd.voc_allele_cache._cache_key;


--
-- Name: voc_annot_count_cache_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.voc_annot_count_cache_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: voc_annot_count_cache; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.voc_annot_count_cache (
    _cache_key integer DEFAULT nextval('mgd.voc_annot_count_cache_seq'::regclass) NOT NULL,
    _term_key integer NOT NULL,
    _mgitype_key integer NOT NULL,
    objectcount integer NOT NULL,
    annotcount integer NOT NULL,
    annottype text NOT NULL
);


--
-- Name: voc_annot_count_cache__cache_key_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.voc_annot_count_cache__cache_key_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: voc_annot_count_cache__cache_key_seq; Type: SEQUENCE OWNED BY; Schema: mgd; Owner: -
--

ALTER SEQUENCE mgd.voc_annot_count_cache__cache_key_seq OWNED BY mgd.voc_annot_count_cache._cache_key;


--
-- Name: voc_term_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: voc_annot_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: voc_evidence_property_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.voc_evidence_property_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: voc_evidence_property; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: voc_evidence_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: voc_go_cache_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.voc_go_cache_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: voc_marker_cache_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.voc_marker_cache_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: voc_marker_cache; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.voc_marker_cache (
    _cache_key integer DEFAULT nextval('mgd.voc_marker_cache_seq'::regclass) NOT NULL,
    _term_key integer NOT NULL,
    _marker_key integer NOT NULL,
    annottype text NOT NULL
);


--
-- Name: voc_marker_cache__cache_key_seq; Type: SEQUENCE; Schema: mgd; Owner: -
--

CREATE SEQUENCE mgd.voc_marker_cache__cache_key_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: voc_marker_cache__cache_key_seq; Type: SEQUENCE OWNED BY; Schema: mgd; Owner: -
--

ALTER SEQUENCE mgd.voc_marker_cache__cache_key_seq OWNED BY mgd.voc_marker_cache._cache_key;


--
-- Name: voc_term_acc_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: voc_term_emapa; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: voc_term_emaps; Type: TABLE; Schema: mgd; Owner: -
--

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


--
-- Name: voc_term_repqualifier_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: voc_termfamily_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: voc_termfamilyedges_view; Type: VIEW; Schema: mgd; Owner: -
--

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


--
-- Name: wks_rosetta; Type: TABLE; Schema: mgd; Owner: -
--

CREATE TABLE mgd.wks_rosetta (
    _rosetta_key integer NOT NULL,
    _marker_key integer,
    wks_markersymbol text,
    wks_markerurl text,
    creation_date timestamp without time zone DEFAULT now() NOT NULL,
    modification_date timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: acc_accession acc_accession_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.acc_accession
    ADD CONSTRAINT acc_accession_pkey PRIMARY KEY (_accession_key);


--
-- Name: acc_accessionmax acc_accessionmax_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.acc_accessionmax
    ADD CONSTRAINT acc_accessionmax_pkey PRIMARY KEY (prefixpart);


--
-- Name: acc_accessionreference acc_accessionreference_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.acc_accessionreference
    ADD CONSTRAINT acc_accessionreference_pkey PRIMARY KEY (_accession_key, _refs_key);


--
-- Name: acc_actualdb acc_actualdb_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.acc_actualdb
    ADD CONSTRAINT acc_actualdb_pkey PRIMARY KEY (_actualdb_key);


--
-- Name: acc_logicaldb acc_logicaldb_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.acc_logicaldb
    ADD CONSTRAINT acc_logicaldb_pkey PRIMARY KEY (_logicaldb_key);


--
-- Name: acc_mgitype acc_mgitype_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.acc_mgitype
    ADD CONSTRAINT acc_mgitype_pkey PRIMARY KEY (_mgitype_key);


--
-- Name: all_allele_cellline all_allele_cellline_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_allele_cellline
    ADD CONSTRAINT all_allele_cellline_pkey PRIMARY KEY (_assoc_key);


--
-- Name: all_allele_mutation all_allele_mutation_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_allele_mutation
    ADD CONSTRAINT all_allele_mutation_pkey PRIMARY KEY (_assoc_key);


--
-- Name: all_allele all_allele_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele_pkey PRIMARY KEY (_allele_key);

ALTER TABLE mgd.all_allele CLUSTER ON all_allele_pkey;


--
-- Name: all_cellline_derivation all_cellline_derivation_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_cellline_derivation
    ADD CONSTRAINT all_cellline_derivation_pkey PRIMARY KEY (_derivation_key);

ALTER TABLE mgd.all_cellline_derivation CLUSTER ON all_cellline_derivation_pkey;


--
-- Name: all_cellline all_cellline_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_cellline
    ADD CONSTRAINT all_cellline_pkey PRIMARY KEY (_cellline_key);


--
-- Name: all_cre_cache all_cre_cache_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_cre_cache
    ADD CONSTRAINT all_cre_cache_pkey PRIMARY KEY (_cache_key);


--
-- Name: all_knockout_cache all_knockout_cache_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_knockout_cache
    ADD CONSTRAINT all_knockout_cache_pkey PRIMARY KEY (_knockout_key);


--
-- Name: all_label all_label_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_label
    ADD CONSTRAINT all_label_pkey PRIMARY KEY (_allele_key, priority, labeltype, label);

ALTER TABLE mgd.all_label CLUSTER ON all_label_pkey;


--
-- Name: all_variant all_variant_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_variant
    ADD CONSTRAINT all_variant_pkey PRIMARY KEY (_variant_key);


--
-- Name: all_variant_sequence all_variant_sequence_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_variant_sequence
    ADD CONSTRAINT all_variant_sequence_pkey PRIMARY KEY (_variantsequence_key);


--
-- Name: bib_books bib_books_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_books
    ADD CONSTRAINT bib_books_pkey PRIMARY KEY (_refs_key);

ALTER TABLE mgd.bib_books CLUSTER ON bib_books_pkey;


--
-- Name: bib_citation_cache bib_citation_cache_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_citation_cache
    ADD CONSTRAINT bib_citation_cache_pkey PRIMARY KEY (_refs_key);

ALTER TABLE mgd.bib_citation_cache CLUSTER ON bib_citation_cache_pkey;


--
-- Name: bib_notes bib_notes_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_notes
    ADD CONSTRAINT bib_notes_pkey PRIMARY KEY (_refs_key);

ALTER TABLE mgd.bib_notes CLUSTER ON bib_notes_pkey;


--
-- Name: bib_refs bib_refs_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_refs
    ADD CONSTRAINT bib_refs_pkey PRIMARY KEY (_refs_key);

ALTER TABLE mgd.bib_refs CLUSTER ON bib_refs_pkey;


--
-- Name: bib_workflow_data bib_workflow_data_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_workflow_data
    ADD CONSTRAINT bib_workflow_data_pkey PRIMARY KEY (_assoc_key);

ALTER TABLE mgd.bib_workflow_data CLUSTER ON bib_workflow_data_pkey;


--
-- Name: bib_workflow_relevance bib_workflow_relevance_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_workflow_relevance
    ADD CONSTRAINT bib_workflow_relevance_pkey PRIMARY KEY (_assoc_key);

ALTER TABLE mgd.bib_workflow_relevance CLUSTER ON bib_workflow_relevance_pkey;


--
-- Name: bib_workflow_status bib_workflow_status_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_workflow_status
    ADD CONSTRAINT bib_workflow_status_pkey PRIMARY KEY (_assoc_key);

ALTER TABLE mgd.bib_workflow_status CLUSTER ON bib_workflow_status_pkey;


--
-- Name: bib_workflow_tag bib_workflow_tag_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_workflow_tag
    ADD CONSTRAINT bib_workflow_tag_pkey PRIMARY KEY (_assoc_key);

ALTER TABLE mgd.bib_workflow_tag CLUSTER ON bib_workflow_tag_pkey;


--
-- Name: crs_cross crs_cross_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.crs_cross
    ADD CONSTRAINT crs_cross_pkey PRIMARY KEY (_cross_key);


--
-- Name: crs_matrix crs_matrix_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.crs_matrix
    ADD CONSTRAINT crs_matrix_pkey PRIMARY KEY (_cross_key, rownumber);


--
-- Name: crs_progeny crs_progeny_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.crs_progeny
    ADD CONSTRAINT crs_progeny_pkey PRIMARY KEY (_cross_key, sequencenum);


--
-- Name: crs_references crs_references_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.crs_references
    ADD CONSTRAINT crs_references_pkey PRIMARY KEY (_cross_key, _marker_key, _refs_key);


--
-- Name: crs_typings crs_typings_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.crs_typings
    ADD CONSTRAINT crs_typings_pkey PRIMARY KEY (_cross_key, rownumber, colnumber);


--
-- Name: dag_closure dag_closure_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.dag_closure
    ADD CONSTRAINT dag_closure_pkey PRIMARY KEY (_dag_key, _ancestor_key, _descendent_key);


--
-- Name: dag_dag dag_dag_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.dag_dag
    ADD CONSTRAINT dag_dag_pkey PRIMARY KEY (_dag_key);


--
-- Name: dag_edge dag_edge_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.dag_edge
    ADD CONSTRAINT dag_edge_pkey PRIMARY KEY (_edge_key);


--
-- Name: dag_label dag_label_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.dag_label
    ADD CONSTRAINT dag_label_pkey PRIMARY KEY (_label_key);


--
-- Name: dag_node dag_node_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.dag_node
    ADD CONSTRAINT dag_node_pkey PRIMARY KEY (_node_key);


--
-- Name: go_tracking go_tracking_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.go_tracking
    ADD CONSTRAINT go_tracking_pkey PRIMARY KEY (_marker_key);


--
-- Name: gxd_allelegenotype gxd_allelegenotype_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_allelegenotype
    ADD CONSTRAINT gxd_allelegenotype_pkey PRIMARY KEY (_genotype_key, _allele_key);


--
-- Name: gxd_allelepair gxd_allelepair_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_allelepair
    ADD CONSTRAINT gxd_allelepair_pkey PRIMARY KEY (_allelepair_key);


--
-- Name: gxd_antibody gxd_antibody_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_antibody
    ADD CONSTRAINT gxd_antibody_pkey PRIMARY KEY (_antibody_key);


--
-- Name: gxd_antibodyalias gxd_antibodyalias_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_antibodyalias
    ADD CONSTRAINT gxd_antibodyalias_pkey PRIMARY KEY (_antibodyalias_key);


--
-- Name: gxd_antibodymarker gxd_antibodymarker_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_antibodymarker
    ADD CONSTRAINT gxd_antibodymarker_pkey PRIMARY KEY (_antibodymarker_key);


--
-- Name: gxd_antibodyprep gxd_antibodyprep_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_antibodyprep
    ADD CONSTRAINT gxd_antibodyprep_pkey PRIMARY KEY (_antibodyprep_key);


--
-- Name: gxd_antigen gxd_antigen_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_antigen
    ADD CONSTRAINT gxd_antigen_pkey PRIMARY KEY (_antigen_key);


--
-- Name: gxd_assay gxd_assay_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_assay
    ADD CONSTRAINT gxd_assay_pkey PRIMARY KEY (_assay_key);


--
-- Name: gxd_assaynote gxd_assaynote_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_assaynote
    ADD CONSTRAINT gxd_assaynote_pkey PRIMARY KEY (_assaynote_key);


--
-- Name: gxd_assaytype gxd_assaytype_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_assaytype
    ADD CONSTRAINT gxd_assaytype_pkey PRIMARY KEY (_assaytype_key);


--
-- Name: gxd_expression gxd_expression_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_expression
    ADD CONSTRAINT gxd_expression_pkey PRIMARY KEY (_expression_key);

ALTER TABLE mgd.gxd_expression CLUSTER ON gxd_expression_pkey;


--
-- Name: gxd_gelband gxd_gelband_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_gelband
    ADD CONSTRAINT gxd_gelband_pkey PRIMARY KEY (_gelband_key);


--
-- Name: gxd_gellane gxd_gellane_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_gellane
    ADD CONSTRAINT gxd_gellane_pkey PRIMARY KEY (_gellane_key);


--
-- Name: gxd_gellanestructure gxd_gellanestructure_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_gellanestructure
    ADD CONSTRAINT gxd_gellanestructure_pkey PRIMARY KEY (_gellanestructure_key);


--
-- Name: gxd_gelrow gxd_gelrow_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_gelrow
    ADD CONSTRAINT gxd_gelrow_pkey PRIMARY KEY (_gelrow_key);


--
-- Name: gxd_genotype gxd_genotype_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_genotype
    ADD CONSTRAINT gxd_genotype_pkey PRIMARY KEY (_genotype_key);


--
-- Name: gxd_htexperiment gxd_htexperiment_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htexperiment
    ADD CONSTRAINT gxd_htexperiment_pkey PRIMARY KEY (_experiment_key);


--
-- Name: gxd_htexperimentvariable gxd_htexperimentvariable_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htexperimentvariable
    ADD CONSTRAINT gxd_htexperimentvariable_pkey PRIMARY KEY (_experimentvariable_key);


--
-- Name: gxd_htrawsample gxd_htrawsample_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htrawsample
    ADD CONSTRAINT gxd_htrawsample_pkey PRIMARY KEY (_rawsample_key);


--
-- Name: gxd_htsample gxd_htsample_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample
    ADD CONSTRAINT gxd_htsample_pkey PRIMARY KEY (_sample_key);


--
-- Name: gxd_htsample_rnaseq gxd_htsample_rnaseq_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseq
    ADD CONSTRAINT gxd_htsample_rnaseq_pkey PRIMARY KEY (_rnaseq_key);


--
-- Name: gxd_htsample_rnaseqcombined gxd_htsample_rnaseqcombined_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseqcombined
    ADD CONSTRAINT gxd_htsample_rnaseqcombined_pkey PRIMARY KEY (_rnaseqcombined_key);


--
-- Name: gxd_htsample_rnaseqset_cache gxd_htsample_rnaseqset_cache_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset_cache
    ADD CONSTRAINT gxd_htsample_rnaseqset_cache_pkey PRIMARY KEY (_assoc_key);


--
-- Name: gxd_htsample_rnaseqset gxd_htsample_rnaseqset_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset
    ADD CONSTRAINT gxd_htsample_rnaseqset_pkey PRIMARY KEY (_rnaseqset_key);


--
-- Name: gxd_htsample_rnaseqsetmember gxd_htsample_rnaseqsetmember_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseqsetmember
    ADD CONSTRAINT gxd_htsample_rnaseqsetmember_pkey PRIMARY KEY (_rnaseqsetmember_key);


--
-- Name: gxd_index gxd_index_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_index
    ADD CONSTRAINT gxd_index_pkey PRIMARY KEY (_index_key);


--
-- Name: gxd_index_stages gxd_index_stages_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_index_stages
    ADD CONSTRAINT gxd_index_stages_pkey PRIMARY KEY (_indexstage_key);


--
-- Name: gxd_insituresult gxd_insituresult_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_insituresult
    ADD CONSTRAINT gxd_insituresult_pkey PRIMARY KEY (_result_key);


--
-- Name: gxd_insituresultimage gxd_insituresultimage_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_insituresultimage
    ADD CONSTRAINT gxd_insituresultimage_pkey PRIMARY KEY (_resultimage_key);


--
-- Name: gxd_isresultcelltype gxd_isresultcelltype_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_isresultcelltype
    ADD CONSTRAINT gxd_isresultcelltype_pkey PRIMARY KEY (_resultcelltype_key);


--
-- Name: gxd_isresultstructure gxd_isresultstructure_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_isresultstructure
    ADD CONSTRAINT gxd_isresultstructure_pkey PRIMARY KEY (_resultstructure_key);


--
-- Name: gxd_probeprep gxd_probeprep_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_probeprep
    ADD CONSTRAINT gxd_probeprep_pkey PRIMARY KEY (_probeprep_key);


--
-- Name: gxd_specimen gxd_specimen_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_specimen
    ADD CONSTRAINT gxd_specimen_pkey PRIMARY KEY (_specimen_key);


--
-- Name: gxd_theilerstage gxd_theilerstage_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_theilerstage
    ADD CONSTRAINT gxd_theilerstage_pkey PRIMARY KEY (_stage_key);


--
-- Name: img_image img_image_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.img_image
    ADD CONSTRAINT img_image_pkey PRIMARY KEY (_image_key);


--
-- Name: img_imagepane_assoc img_imagepane_assoc_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.img_imagepane_assoc
    ADD CONSTRAINT img_imagepane_assoc_pkey PRIMARY KEY (_assoc_key);


--
-- Name: img_imagepane img_imagepane_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.img_imagepane
    ADD CONSTRAINT img_imagepane_pkey PRIMARY KEY (_imagepane_key);


--
-- Name: map_coord_collection map_coord_collection_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.map_coord_collection
    ADD CONSTRAINT map_coord_collection_pkey PRIMARY KEY (_collection_key);


--
-- Name: map_coord_feature map_coord_feature_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.map_coord_feature
    ADD CONSTRAINT map_coord_feature_pkey PRIMARY KEY (_feature_key);


--
-- Name: map_coordinate map_coordinate_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.map_coordinate
    ADD CONSTRAINT map_coordinate_pkey PRIMARY KEY (_map_key);


--
-- Name: mgi_dbinfo mgi_dbinfo_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_dbinfo
    ADD CONSTRAINT mgi_dbinfo_pkey PRIMARY KEY (public_version);


--
-- Name: mgi_keyvalue mgi_keyvalue_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_keyvalue
    ADD CONSTRAINT mgi_keyvalue_pkey PRIMARY KEY (_keyvalue_key);

ALTER TABLE mgd.mgi_keyvalue CLUSTER ON mgi_keyvalue_pkey;


--
-- Name: mgi_note mgi_note_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_note
    ADD CONSTRAINT mgi_note_pkey PRIMARY KEY (_note_key);

ALTER TABLE mgd.mgi_note CLUSTER ON mgi_note_pkey;


--
-- Name: mgi_notetype mgi_notetype_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_notetype
    ADD CONSTRAINT mgi_notetype_pkey PRIMARY KEY (_notetype_key);


--
-- Name: mgi_organism_mgitype mgi_organism_mgitype_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_organism_mgitype
    ADD CONSTRAINT mgi_organism_mgitype_pkey PRIMARY KEY (_assoc_key);


--
-- Name: mgi_organism mgi_organism_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_organism
    ADD CONSTRAINT mgi_organism_pkey PRIMARY KEY (_organism_key);


--
-- Name: mgi_property mgi_property_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_property
    ADD CONSTRAINT mgi_property_pkey PRIMARY KEY (_property_key);


--
-- Name: mgi_propertytype mgi_propertytype_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_propertytype
    ADD CONSTRAINT mgi_propertytype_pkey PRIMARY KEY (_propertytype_key);


--
-- Name: mgi_refassoctype mgi_refassoctype_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_refassoctype
    ADD CONSTRAINT mgi_refassoctype_pkey PRIMARY KEY (_refassoctype_key);


--
-- Name: mgi_reference_assoc mgi_reference_assoc_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_reference_assoc
    ADD CONSTRAINT mgi_reference_assoc_pkey PRIMARY KEY (_assoc_key);


--
-- Name: mgi_relationship_category mgi_relationship_category_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_relationship_category
    ADD CONSTRAINT mgi_relationship_category_pkey PRIMARY KEY (_category_key);

ALTER TABLE mgd.mgi_relationship_category CLUSTER ON mgi_relationship_category_pkey;


--
-- Name: mgi_relationship mgi_relationship_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_relationship
    ADD CONSTRAINT mgi_relationship_pkey PRIMARY KEY (_relationship_key);

ALTER TABLE mgd.mgi_relationship CLUSTER ON mgi_relationship_pkey;


--
-- Name: mgi_relationship_property mgi_relationship_property_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_relationship_property
    ADD CONSTRAINT mgi_relationship_property_pkey PRIMARY KEY (_relationshipproperty_key);

ALTER TABLE mgd.mgi_relationship_property CLUSTER ON mgi_relationship_property_pkey;


--
-- Name: mgi_set mgi_set_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_set
    ADD CONSTRAINT mgi_set_pkey PRIMARY KEY (_set_key);


--
-- Name: mgi_setmember_emapa mgi_setmember_emapa_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_setmember_emapa
    ADD CONSTRAINT mgi_setmember_emapa_pkey PRIMARY KEY (_setmember_emapa_key);


--
-- Name: mgi_setmember mgi_setmember_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_setmember
    ADD CONSTRAINT mgi_setmember_pkey PRIMARY KEY (_setmember_key);


--
-- Name: mgi_synonym mgi_synonym_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_synonym
    ADD CONSTRAINT mgi_synonym_pkey PRIMARY KEY (_synonym_key);


--
-- Name: mgi_synonymtype mgi_synonymtype_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_synonymtype
    ADD CONSTRAINT mgi_synonymtype_pkey PRIMARY KEY (_synonymtype_key);


--
-- Name: mgi_translation mgi_translation_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_translation
    ADD CONSTRAINT mgi_translation_pkey PRIMARY KEY (_translation_key);


--
-- Name: mgi_translationtype mgi_translationtype_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_translationtype
    ADD CONSTRAINT mgi_translationtype_pkey PRIMARY KEY (_translationtype_key);


--
-- Name: mgi_user mgi_user_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_user
    ADD CONSTRAINT mgi_user_pkey PRIMARY KEY (_user_key);


--
-- Name: mld_assay_types mld_assay_types_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_assay_types
    ADD CONSTRAINT mld_assay_types_pkey PRIMARY KEY (_assay_type_key);

ALTER TABLE mgd.mld_assay_types CLUSTER ON mld_assay_types_pkey;


--
-- Name: mld_concordance mld_concordance_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_concordance
    ADD CONSTRAINT mld_concordance_pkey PRIMARY KEY (_expt_key, sequencenum);

ALTER TABLE mgd.mld_concordance CLUSTER ON mld_concordance_pkey;


--
-- Name: mld_contig mld_contig_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_contig
    ADD CONSTRAINT mld_contig_pkey PRIMARY KEY (_contig_key);

ALTER TABLE mgd.mld_contig CLUSTER ON mld_contig_pkey;


--
-- Name: mld_contigprobe mld_contigprobe_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_contigprobe
    ADD CONSTRAINT mld_contigprobe_pkey PRIMARY KEY (_contig_key, sequencenum);

ALTER TABLE mgd.mld_contigprobe CLUSTER ON mld_contigprobe_pkey;


--
-- Name: mld_expt_marker mld_expt_marker_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_expt_marker
    ADD CONSTRAINT mld_expt_marker_pkey PRIMARY KEY (_assoc_key);

ALTER TABLE mgd.mld_expt_marker CLUSTER ON mld_expt_marker_pkey;


--
-- Name: mld_expt_notes mld_expt_notes_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_expt_notes
    ADD CONSTRAINT mld_expt_notes_pkey PRIMARY KEY (_expt_key);

ALTER TABLE mgd.mld_expt_notes CLUSTER ON mld_expt_notes_pkey;


--
-- Name: mld_expts mld_expts_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_expts
    ADD CONSTRAINT mld_expts_pkey PRIMARY KEY (_expt_key);

ALTER TABLE mgd.mld_expts CLUSTER ON mld_expts_pkey;


--
-- Name: mld_fish mld_fish_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_fish
    ADD CONSTRAINT mld_fish_pkey PRIMARY KEY (_expt_key);

ALTER TABLE mgd.mld_fish CLUSTER ON mld_fish_pkey;


--
-- Name: mld_fish_region mld_fish_region_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_fish_region
    ADD CONSTRAINT mld_fish_region_pkey PRIMARY KEY (_expt_key, sequencenum);

ALTER TABLE mgd.mld_fish_region CLUSTER ON mld_fish_region_pkey;


--
-- Name: mld_hit mld_hit_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_hit
    ADD CONSTRAINT mld_hit_pkey PRIMARY KEY (_expt_key, _probe_key, _target_key);

ALTER TABLE mgd.mld_hit CLUSTER ON mld_hit_pkey;


--
-- Name: mld_hybrid mld_hybrid_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_hybrid
    ADD CONSTRAINT mld_hybrid_pkey PRIMARY KEY (_expt_key);

ALTER TABLE mgd.mld_hybrid CLUSTER ON mld_hybrid_pkey;


--
-- Name: mld_insitu mld_insitu_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_insitu
    ADD CONSTRAINT mld_insitu_pkey PRIMARY KEY (_expt_key);

ALTER TABLE mgd.mld_insitu CLUSTER ON mld_insitu_pkey;


--
-- Name: mld_isregion mld_isregion_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_isregion
    ADD CONSTRAINT mld_isregion_pkey PRIMARY KEY (_expt_key, sequencenum);

ALTER TABLE mgd.mld_isregion CLUSTER ON mld_isregion_pkey;


--
-- Name: mld_matrix mld_matrix_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_matrix
    ADD CONSTRAINT mld_matrix_pkey PRIMARY KEY (_expt_key);

ALTER TABLE mgd.mld_matrix CLUSTER ON mld_matrix_pkey;


--
-- Name: mld_mc2point mld_mc2point_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_mc2point
    ADD CONSTRAINT mld_mc2point_pkey PRIMARY KEY (_expt_key, sequencenum);

ALTER TABLE mgd.mld_mc2point CLUSTER ON mld_mc2point_pkey;


--
-- Name: mld_mcdatalist mld_mcdatalist_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_mcdatalist
    ADD CONSTRAINT mld_mcdatalist_pkey PRIMARY KEY (_expt_key, sequencenum);

ALTER TABLE mgd.mld_mcdatalist CLUSTER ON mld_mcdatalist_pkey;


--
-- Name: mld_notes mld_notes_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_notes
    ADD CONSTRAINT mld_notes_pkey PRIMARY KEY (_refs_key);

ALTER TABLE mgd.mld_notes CLUSTER ON mld_notes_pkey;


--
-- Name: mld_ri2point mld_ri2point_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_ri2point
    ADD CONSTRAINT mld_ri2point_pkey PRIMARY KEY (_expt_key, sequencenum);

ALTER TABLE mgd.mld_ri2point CLUSTER ON mld_ri2point_pkey;


--
-- Name: mld_ri mld_ri_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_ri
    ADD CONSTRAINT mld_ri_pkey PRIMARY KEY (_expt_key);

ALTER TABLE mgd.mld_ri CLUSTER ON mld_ri_pkey;


--
-- Name: mld_ridata mld_ridata_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_ridata
    ADD CONSTRAINT mld_ridata_pkey PRIMARY KEY (_expt_key, sequencenum);

ALTER TABLE mgd.mld_ridata CLUSTER ON mld_ridata_pkey;


--
-- Name: mld_statistics mld_statistics_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_statistics
    ADD CONSTRAINT mld_statistics_pkey PRIMARY KEY (_expt_key, sequencenum);

ALTER TABLE mgd.mld_statistics CLUSTER ON mld_statistics_pkey;


--
-- Name: mrk_biotypemapping mrk_biotypemapping_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_biotypemapping
    ADD CONSTRAINT mrk_biotypemapping_pkey PRIMARY KEY (_biotypemapping_key);


--
-- Name: mrk_chromosome mrk_chromosome_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_chromosome
    ADD CONSTRAINT mrk_chromosome_pkey PRIMARY KEY (_chromosome_key);


--
-- Name: mrk_cluster mrk_cluster_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_cluster
    ADD CONSTRAINT mrk_cluster_pkey PRIMARY KEY (_cluster_key);


--
-- Name: mrk_clustermember mrk_clustermember_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_clustermember
    ADD CONSTRAINT mrk_clustermember_pkey PRIMARY KEY (_clustermember_key);


--
-- Name: mrk_current mrk_current_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_current
    ADD CONSTRAINT mrk_current_pkey PRIMARY KEY (_current_key, _marker_key);


--
-- Name: mrk_do_cache mrk_do_cache_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_do_cache
    ADD CONSTRAINT mrk_do_cache_pkey PRIMARY KEY (_cache_key);

ALTER TABLE mgd.mrk_do_cache CLUSTER ON mrk_do_cache_pkey;


--
-- Name: mrk_history mrk_history_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_history
    ADD CONSTRAINT mrk_history_pkey PRIMARY KEY (_assoc_key);

ALTER TABLE mgd.mrk_history CLUSTER ON mrk_history_pkey;


--
-- Name: mrk_label mrk_label_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_label
    ADD CONSTRAINT mrk_label_pkey PRIMARY KEY (_label_key);

ALTER TABLE mgd.mrk_label CLUSTER ON mrk_label_pkey;


--
-- Name: mrk_location_cache mrk_location_cache_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_location_cache
    ADD CONSTRAINT mrk_location_cache_pkey PRIMARY KEY (_marker_key);

ALTER TABLE mgd.mrk_location_cache CLUSTER ON mrk_location_cache_pkey;


--
-- Name: mrk_marker mrk_marker_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_marker
    ADD CONSTRAINT mrk_marker_pkey PRIMARY KEY (_marker_key);


--
-- Name: mrk_mcv_cache mrk_mcv_cache_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_mcv_cache
    ADD CONSTRAINT mrk_mcv_cache_pkey PRIMARY KEY (_marker_key, _mcvterm_key);

ALTER TABLE mgd.mrk_mcv_cache CLUSTER ON mrk_mcv_cache_pkey;


--
-- Name: mrk_mcv_count_cache mrk_mcv_count_cache_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_mcv_count_cache
    ADD CONSTRAINT mrk_mcv_count_cache_pkey PRIMARY KEY (_mcvterm_key);

ALTER TABLE mgd.mrk_mcv_count_cache CLUSTER ON mrk_mcv_count_cache_pkey;


--
-- Name: mrk_notes mrk_notes_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_notes
    ADD CONSTRAINT mrk_notes_pkey PRIMARY KEY (_marker_key);


--
-- Name: mrk_reference mrk_reference_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_reference
    ADD CONSTRAINT mrk_reference_pkey PRIMARY KEY (_marker_key, _refs_key);

ALTER TABLE mgd.mrk_reference CLUSTER ON mrk_reference_pkey;


--
-- Name: mrk_status mrk_status_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_status
    ADD CONSTRAINT mrk_status_pkey PRIMARY KEY (_marker_status_key);


--
-- Name: mrk_strainmarker mrk_strainmarker_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_strainmarker
    ADD CONSTRAINT mrk_strainmarker_pkey PRIMARY KEY (_strainmarker_key);


--
-- Name: mrk_types mrk_types_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_types
    ADD CONSTRAINT mrk_types_pkey PRIMARY KEY (_marker_type_key);


--
-- Name: prb_alias prb_alias_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_alias
    ADD CONSTRAINT prb_alias_pkey PRIMARY KEY (_alias_key);

ALTER TABLE mgd.prb_alias CLUSTER ON prb_alias_pkey;


--
-- Name: prb_allele prb_allele_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_allele
    ADD CONSTRAINT prb_allele_pkey PRIMARY KEY (_allele_key);


--
-- Name: prb_allele_strain prb_allele_strain_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_allele_strain
    ADD CONSTRAINT prb_allele_strain_pkey PRIMARY KEY (_allelestrain_key);


--
-- Name: prb_marker prb_marker_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_marker
    ADD CONSTRAINT prb_marker_pkey PRIMARY KEY (_assoc_key);

ALTER TABLE mgd.prb_marker CLUSTER ON prb_marker_pkey;


--
-- Name: prb_notes prb_notes_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_notes
    ADD CONSTRAINT prb_notes_pkey PRIMARY KEY (_note_key);

ALTER TABLE mgd.prb_notes CLUSTER ON prb_notes_pkey;


--
-- Name: prb_probe prb_probe_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_probe
    ADD CONSTRAINT prb_probe_pkey PRIMARY KEY (_probe_key);

ALTER TABLE mgd.prb_probe CLUSTER ON prb_probe_pkey;


--
-- Name: prb_ref_notes prb_ref_notes_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_ref_notes
    ADD CONSTRAINT prb_ref_notes_pkey PRIMARY KEY (_reference_key);

ALTER TABLE mgd.prb_ref_notes CLUSTER ON prb_ref_notes_pkey;


--
-- Name: prb_reference prb_reference_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_reference
    ADD CONSTRAINT prb_reference_pkey PRIMARY KEY (_reference_key);

ALTER TABLE mgd.prb_reference CLUSTER ON prb_reference_pkey;


--
-- Name: prb_rflv prb_rflv_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_rflv
    ADD CONSTRAINT prb_rflv_pkey PRIMARY KEY (_rflv_key);

ALTER TABLE mgd.prb_rflv CLUSTER ON prb_rflv_pkey;


--
-- Name: prb_source prb_source_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_source
    ADD CONSTRAINT prb_source_pkey PRIMARY KEY (_source_key);


--
-- Name: prb_strain_genotype prb_strain_genotype_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_strain_genotype
    ADD CONSTRAINT prb_strain_genotype_pkey PRIMARY KEY (_straingenotype_key);

ALTER TABLE mgd.prb_strain_genotype CLUSTER ON prb_strain_genotype_pkey;


--
-- Name: prb_strain_marker prb_strain_marker_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_strain_marker
    ADD CONSTRAINT prb_strain_marker_pkey PRIMARY KEY (_strainmarker_key);

ALTER TABLE mgd.prb_strain_marker CLUSTER ON prb_strain_marker_pkey;


--
-- Name: prb_strain prb_strain_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_strain
    ADD CONSTRAINT prb_strain_pkey PRIMARY KEY (_strain_key);


--
-- Name: prb_tissue prb_tissue_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_tissue
    ADD CONSTRAINT prb_tissue_pkey PRIMARY KEY (_tissue_key);

ALTER TABLE mgd.prb_tissue CLUSTER ON prb_tissue_pkey;


--
-- Name: ri_riset ri_riset_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.ri_riset
    ADD CONSTRAINT ri_riset_pkey PRIMARY KEY (_riset_key);


--
-- Name: ri_summary_expt_ref ri_summary_expt_ref_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.ri_summary_expt_ref
    ADD CONSTRAINT ri_summary_expt_ref_pkey PRIMARY KEY (_risummary_key, _expt_key);


--
-- Name: ri_summary ri_summary_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.ri_summary
    ADD CONSTRAINT ri_summary_pkey PRIMARY KEY (_risummary_key);


--
-- Name: seq_allele_assoc seq_allele_assoc_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_allele_assoc
    ADD CONSTRAINT seq_allele_assoc_pkey PRIMARY KEY (_assoc_key);


--
-- Name: seq_coord_cache seq_coord_cache_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_coord_cache
    ADD CONSTRAINT seq_coord_cache_pkey PRIMARY KEY (_map_key, _sequence_key);

ALTER TABLE mgd.seq_coord_cache CLUSTER ON seq_coord_cache_pkey;


--
-- Name: seq_genemodel seq_genemodel_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_genemodel
    ADD CONSTRAINT seq_genemodel_pkey PRIMARY KEY (_sequence_key);


--
-- Name: seq_genetrap seq_genetrap_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_genetrap
    ADD CONSTRAINT seq_genetrap_pkey PRIMARY KEY (_sequence_key);


--
-- Name: seq_marker_cache seq_marker_cache_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_marker_cache
    ADD CONSTRAINT seq_marker_cache_pkey PRIMARY KEY (_cache_key);

ALTER TABLE mgd.seq_marker_cache CLUSTER ON seq_marker_cache_pkey;


--
-- Name: seq_probe_cache seq_probe_cache_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_probe_cache
    ADD CONSTRAINT seq_probe_cache_pkey PRIMARY KEY (_sequence_key, _probe_key, _refs_key);

ALTER TABLE mgd.seq_probe_cache CLUSTER ON seq_probe_cache_pkey;


--
-- Name: seq_sequence_assoc seq_sequence_assoc_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_sequence_assoc
    ADD CONSTRAINT seq_sequence_assoc_pkey PRIMARY KEY (_assoc_key);


--
-- Name: seq_sequence seq_sequence_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_sequence
    ADD CONSTRAINT seq_sequence_pkey PRIMARY KEY (_sequence_key);

ALTER TABLE mgd.seq_sequence CLUSTER ON seq_sequence_pkey;


--
-- Name: seq_sequence_raw seq_sequence_raw_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_sequence_raw
    ADD CONSTRAINT seq_sequence_raw_pkey PRIMARY KEY (_sequence_key);


--
-- Name: seq_source_assoc seq_source_assoc_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_source_assoc
    ADD CONSTRAINT seq_source_assoc_pkey PRIMARY KEY (_assoc_key);


--
-- Name: voc_allele_cache voc_allele_cache_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_allele_cache
    ADD CONSTRAINT voc_allele_cache_pkey PRIMARY KEY (_cache_key);


--
-- Name: voc_annot_count_cache voc_annot_count_cache_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_annot_count_cache
    ADD CONSTRAINT voc_annot_count_cache_pkey PRIMARY KEY (_cache_key);


--
-- Name: voc_annot voc_annot_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_annot
    ADD CONSTRAINT voc_annot_pkey PRIMARY KEY (_annot_key);


--
-- Name: voc_annotheader voc_annotheader_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_annotheader
    ADD CONSTRAINT voc_annotheader_pkey PRIMARY KEY (_annotheader_key);


--
-- Name: voc_annottype voc_annottype_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_annottype
    ADD CONSTRAINT voc_annottype_pkey PRIMARY KEY (_annottype_key);


--
-- Name: voc_evidence voc_evidence_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_evidence
    ADD CONSTRAINT voc_evidence_pkey PRIMARY KEY (_annotevidence_key);


--
-- Name: voc_evidence_property voc_evidence_property_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_evidence_property
    ADD CONSTRAINT voc_evidence_property_pkey PRIMARY KEY (_evidenceproperty_key);

ALTER TABLE mgd.voc_evidence_property CLUSTER ON voc_evidence_property_pkey;


--
-- Name: voc_marker_cache voc_marker_cache_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_marker_cache
    ADD CONSTRAINT voc_marker_cache_pkey PRIMARY KEY (_cache_key);


--
-- Name: voc_term_emapa voc_term_emapa_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_term_emapa
    ADD CONSTRAINT voc_term_emapa_pkey PRIMARY KEY (_term_key);


--
-- Name: voc_term_emaps voc_term_emaps_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_term_emaps
    ADD CONSTRAINT voc_term_emaps_pkey PRIMARY KEY (_term_key);


--
-- Name: voc_term voc_term_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_term
    ADD CONSTRAINT voc_term_pkey PRIMARY KEY (_term_key);


--
-- Name: voc_vocab voc_vocab_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_vocab
    ADD CONSTRAINT voc_vocab_pkey PRIMARY KEY (_vocab_key);


--
-- Name: voc_vocabdag voc_vocabdag_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_vocabdag
    ADD CONSTRAINT voc_vocabdag_pkey PRIMARY KEY (_vocab_key, _dag_key);


--
-- Name: wks_rosetta wks_rosetta_pkey; Type: CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.wks_rosetta
    ADD CONSTRAINT wks_rosetta_pkey PRIMARY KEY (_rosetta_key);


--
-- Name: acc_accession_0; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX acc_accession_0 ON mgd.acc_accession USING btree (lower(accid));


--
-- Name: acc_accession_1; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX acc_accession_1 ON mgd.acc_accession USING btree (lower(prefixpart));


--
-- Name: acc_accession_idx_accid; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX acc_accession_idx_accid ON mgd.acc_accession USING btree (accid);


--
-- Name: acc_accession_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX acc_accession_idx_clustered ON mgd.acc_accession USING btree (_object_key, _mgitype_key);


--
-- Name: acc_accession_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX acc_accession_idx_createdby_key ON mgd.acc_accession USING btree (_createdby_key);


--
-- Name: acc_accession_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX acc_accession_idx_creation_date ON mgd.acc_accession USING btree (creation_date);


--
-- Name: acc_accession_idx_logicaldb_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX acc_accession_idx_logicaldb_key ON mgd.acc_accession USING btree (_logicaldb_key);


--
-- Name: acc_accession_idx_mgitype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX acc_accession_idx_mgitype_key ON mgd.acc_accession USING btree (_mgitype_key);


--
-- Name: acc_accession_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX acc_accession_idx_modification_date ON mgd.acc_accession USING btree (modification_date);


--
-- Name: acc_accession_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX acc_accession_idx_modifiedby_key ON mgd.acc_accession USING btree (_modifiedby_key);


--
-- Name: acc_accession_idx_numericpart; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX acc_accession_idx_numericpart ON mgd.acc_accession USING btree (numericpart);


--
-- Name: acc_accession_idx_prefixpart; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX acc_accession_idx_prefixpart ON mgd.acc_accession USING btree (prefixpart);


--
-- Name: acc_accessionreference_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX acc_accessionreference_idx_createdby_key ON mgd.acc_accessionreference USING btree (_createdby_key);


--
-- Name: acc_accessionreference_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX acc_accessionreference_idx_creation_date ON mgd.acc_accessionreference USING btree (creation_date);


--
-- Name: acc_accessionreference_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX acc_accessionreference_idx_modification_date ON mgd.acc_accessionreference USING btree (modification_date);


--
-- Name: acc_accessionreference_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX acc_accessionreference_idx_modifiedby_key ON mgd.acc_accessionreference USING btree (_modifiedby_key);


--
-- Name: acc_accessionreference_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX acc_accessionreference_idx_refs_key ON mgd.acc_accessionreference USING btree (_refs_key);


--
-- Name: acc_actualdb_idx_logicaldb_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX acc_actualdb_idx_logicaldb_key ON mgd.acc_actualdb USING btree (_logicaldb_key);


--
-- Name: acc_actualdb_idx_name; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX acc_actualdb_idx_name ON mgd.acc_actualdb USING btree (name);


--
-- Name: acc_logicaldb_idx_name; Type: INDEX; Schema: mgd; Owner: -
--

CREATE UNIQUE INDEX acc_logicaldb_idx_name ON mgd.acc_logicaldb USING btree (name, _logicaldb_key);


--
-- Name: acc_logicaldb_idx_organism_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX acc_logicaldb_idx_organism_key ON mgd.acc_logicaldb USING btree (_organism_key);


--
-- Name: acc_mgitype_0; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX acc_mgitype_0 ON mgd.acc_mgitype USING btree (lower(name));


--
-- Name: acc_mgitype_idx_name; Type: INDEX; Schema: mgd; Owner: -
--

CREATE UNIQUE INDEX acc_mgitype_idx_name ON mgd.acc_mgitype USING btree (name);


--
-- Name: all_allele_cellline_idx_allele_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_allele_cellline_idx_allele_key ON mgd.all_allele_cellline USING btree (_allele_key, _mutantcellline_key);


--
-- Name: all_allele_cellline_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_allele_cellline_idx_createdby_key ON mgd.all_allele_cellline USING btree (_createdby_key);


--
-- Name: all_allele_cellline_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_allele_cellline_idx_creation_date ON mgd.all_allele_cellline USING btree (creation_date);


--
-- Name: all_allele_cellline_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_allele_cellline_idx_modification_date ON mgd.all_allele_cellline USING btree (modification_date);


--
-- Name: all_allele_cellline_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_allele_cellline_idx_modifiedby_key ON mgd.all_allele_cellline USING btree (_modifiedby_key);


--
-- Name: all_allele_cellline_idx_mutantcellline_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_allele_cellline_idx_mutantcellline_key ON mgd.all_allele_cellline USING btree (_mutantcellline_key);


--
-- Name: all_allele_idx_allele_status_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_allele_idx_allele_status_key ON mgd.all_allele USING btree (_allele_status_key);


--
-- Name: all_allele_idx_allele_type_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_allele_idx_allele_type_key ON mgd.all_allele USING btree (_allele_type_key);


--
-- Name: all_allele_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_allele_idx_clustered ON mgd.all_allele USING btree (_marker_key);


--
-- Name: all_allele_idx_collection_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_allele_idx_collection_key ON mgd.all_allele USING btree (_collection_key);


--
-- Name: all_allele_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_allele_idx_createdby_key ON mgd.all_allele USING btree (_createdby_key);


--
-- Name: all_allele_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_allele_idx_creation_date ON mgd.all_allele USING btree (creation_date);


--
-- Name: all_allele_idx_markerallele_status_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_allele_idx_markerallele_status_key ON mgd.all_allele USING btree (_markerallele_status_key);


--
-- Name: all_allele_idx_mode_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_allele_idx_mode_key ON mgd.all_allele USING btree (_mode_key);


--
-- Name: all_allele_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_allele_idx_modification_date ON mgd.all_allele USING btree (modification_date);


--
-- Name: all_allele_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_allele_idx_modifiedby_key ON mgd.all_allele USING btree (_modifiedby_key);


--
-- Name: all_allele_idx_name; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_allele_idx_name ON mgd.all_allele USING btree (name);


--
-- Name: all_allele_idx_strain_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_allele_idx_strain_key ON mgd.all_allele USING btree (_strain_key);


--
-- Name: all_allele_idx_symbol; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_allele_idx_symbol ON mgd.all_allele USING btree (symbol);


--
-- Name: all_allele_idx_transmission_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_allele_idx_transmission_key ON mgd.all_allele USING btree (_transmission_key);


--
-- Name: all_allele_mutation_idx_allele_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_allele_mutation_idx_allele_key ON mgd.all_allele_mutation USING btree (_allele_key);


--
-- Name: all_allele_mutation_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_allele_mutation_idx_creation_date ON mgd.all_allele_mutation USING btree (creation_date);


--
-- Name: all_allele_mutation_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_allele_mutation_idx_modification_date ON mgd.all_allele_mutation USING btree (modification_date);


--
-- Name: all_allele_mutation_idx_mutation_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_allele_mutation_idx_mutation_key ON mgd.all_allele_mutation USING btree (_mutation_key);


--
-- Name: all_cellline_derivation_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cellline_derivation_idx_createdby_key ON mgd.all_cellline_derivation USING btree (_createdby_key);


--
-- Name: all_cellline_derivation_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cellline_derivation_idx_creation_date ON mgd.all_cellline_derivation USING btree (creation_date);


--
-- Name: all_cellline_derivation_idx_creator_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cellline_derivation_idx_creator_key ON mgd.all_cellline_derivation USING btree (_creator_key);


--
-- Name: all_cellline_derivation_idx_derivationtype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cellline_derivation_idx_derivationtype_key ON mgd.all_cellline_derivation USING btree (_derivationtype_key, _parentcellline_key, _creator_key);


--
-- Name: all_cellline_derivation_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cellline_derivation_idx_modification_date ON mgd.all_cellline_derivation USING btree (modification_date);


--
-- Name: all_cellline_derivation_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cellline_derivation_idx_modifiedby_key ON mgd.all_cellline_derivation USING btree (_modifiedby_key);


--
-- Name: all_cellline_derivation_idx_name; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cellline_derivation_idx_name ON mgd.all_cellline_derivation USING btree (name, _derivation_key);


--
-- Name: all_cellline_derivation_idx_pcl; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cellline_derivation_idx_pcl ON mgd.all_cellline_derivation USING btree (_parentcellline_key, _derivation_key);


--
-- Name: all_cellline_derivation_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cellline_derivation_idx_refs_key ON mgd.all_cellline_derivation USING btree (_refs_key);


--
-- Name: all_cellline_derivation_idx_vector_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cellline_derivation_idx_vector_key ON mgd.all_cellline_derivation USING btree (_vector_key);


--
-- Name: all_cellline_derivation_idx_vectortype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cellline_derivation_idx_vectortype_key ON mgd.all_cellline_derivation USING btree (_vectortype_key);


--
-- Name: all_cellline_idx_cellline; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cellline_idx_cellline ON mgd.all_cellline USING btree (cellline);


--
-- Name: all_cellline_idx_cellline_type_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cellline_idx_cellline_type_key ON mgd.all_cellline USING btree (_cellline_type_key, _derivation_key, _cellline_key);


--
-- Name: all_cellline_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cellline_idx_createdby_key ON mgd.all_cellline USING btree (_createdby_key);


--
-- Name: all_cellline_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cellline_idx_creation_date ON mgd.all_cellline USING btree (creation_date);


--
-- Name: all_cellline_idx_derivation_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cellline_idx_derivation_key ON mgd.all_cellline USING btree (_derivation_key, _cellline_key);


--
-- Name: all_cellline_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cellline_idx_modification_date ON mgd.all_cellline USING btree (modification_date);


--
-- Name: all_cellline_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cellline_idx_modifiedby_key ON mgd.all_cellline USING btree (_modifiedby_key);


--
-- Name: all_cellline_idx_strain_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cellline_idx_strain_key ON mgd.all_cellline USING btree (_strain_key);


--
-- Name: all_cre_cache_idx_allele_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cre_cache_idx_allele_key ON mgd.all_cre_cache USING btree (_allele_key);


--
-- Name: all_cre_cache_idx_allele_type_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cre_cache_idx_allele_type_key ON mgd.all_cre_cache USING btree (_allele_type_key);


--
-- Name: all_cre_cache_idx_assay_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cre_cache_idx_assay_key ON mgd.all_cre_cache USING btree (_assay_key);


--
-- Name: all_cre_cache_idx_celltype_term_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cre_cache_idx_celltype_term_key ON mgd.all_cre_cache USING btree (_celltype_term_key);


--
-- Name: all_cre_cache_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cre_cache_idx_clustered ON mgd.all_cre_cache USING btree (_allele_key, _emapa_term_key, _stage_key, _assay_key, expressed);

ALTER TABLE mgd.all_cre_cache CLUSTER ON all_cre_cache_idx_clustered;


--
-- Name: all_cre_cache_idx_emapa_term_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cre_cache_idx_emapa_term_key ON mgd.all_cre_cache USING btree (_emapa_term_key);


--
-- Name: all_cre_cache_idx_stage_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_cre_cache_idx_stage_key ON mgd.all_cre_cache USING btree (_stage_key);


--
-- Name: all_knockout_cache_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_knockout_cache_idx_clustered ON mgd.all_knockout_cache USING btree (_allele_key);

ALTER TABLE mgd.all_knockout_cache CLUSTER ON all_knockout_cache_idx_clustered;


--
-- Name: all_knockout_cache_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE UNIQUE INDEX all_knockout_cache_idx_marker_key ON mgd.all_knockout_cache USING btree (_marker_key);


--
-- Name: all_label_0; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_label_0 ON mgd.all_label USING btree (lower(label));


--
-- Name: all_label_idx_label; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_label_idx_label ON mgd.all_label USING btree (label);


--
-- Name: all_label_idx_label_status_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_label_idx_label_status_key ON mgd.all_label USING btree (_label_status_key);


--
-- Name: all_label_idx_priority; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_label_idx_priority ON mgd.all_label USING btree (priority);


--
-- Name: all_variant_idx_allele_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_variant_idx_allele_key ON mgd.all_variant USING btree (_allele_key);


--
-- Name: all_variant_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_variant_idx_createdby_key ON mgd.all_variant USING btree (_createdby_key);


--
-- Name: all_variant_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_variant_idx_creation_date ON mgd.all_variant USING btree (creation_date);


--
-- Name: all_variant_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_variant_idx_modification_date ON mgd.all_variant USING btree (modification_date);


--
-- Name: all_variant_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_variant_idx_modifiedby_key ON mgd.all_variant USING btree (_modifiedby_key);


--
-- Name: all_variant_idx_sourcevariant_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_variant_idx_sourcevariant_key ON mgd.all_variant USING btree (_sourcevariant_key);


--
-- Name: all_variant_idx_strain_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_variant_idx_strain_key ON mgd.all_variant USING btree (_strain_key);


--
-- Name: all_variant_sequence_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_variant_sequence_idx_createdby_key ON mgd.all_variant_sequence USING btree (_createdby_key);


--
-- Name: all_variant_sequence_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_variant_sequence_idx_creation_date ON mgd.all_variant_sequence USING btree (creation_date);


--
-- Name: all_variant_sequence_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_variant_sequence_idx_modification_date ON mgd.all_variant_sequence USING btree (modification_date);


--
-- Name: all_variant_sequence_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_variant_sequence_idx_modifiedby_key ON mgd.all_variant_sequence USING btree (_modifiedby_key);


--
-- Name: all_variant_sequence_idx_sequence_type_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_variant_sequence_idx_sequence_type_key ON mgd.all_variant_sequence USING btree (_sequence_type_key);


--
-- Name: all_variant_sequence_idx_variant_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX all_variant_sequence_idx_variant_key ON mgd.all_variant_sequence USING btree (_variant_key);


--
-- Name: bib_books_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_books_idx_creation_date ON mgd.bib_books USING btree (creation_date);


--
-- Name: bib_books_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_books_idx_modification_date ON mgd.bib_books USING btree (modification_date);


--
-- Name: bib_citation_cache_idx_doiid; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_citation_cache_idx_doiid ON mgd.bib_citation_cache USING btree (doiid);


--
-- Name: bib_citation_cache_idx_jnumid; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_citation_cache_idx_jnumid ON mgd.bib_citation_cache USING btree (jnumid);


--
-- Name: bib_citation_cache_idx_journal; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_citation_cache_idx_journal ON mgd.bib_citation_cache USING btree (journal);


--
-- Name: bib_citation_cache_idx_mgiid; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_citation_cache_idx_mgiid ON mgd.bib_citation_cache USING btree (mgiid);


--
-- Name: bib_citation_cache_idx_numericpart; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_citation_cache_idx_numericpart ON mgd.bib_citation_cache USING btree (numericpart);


--
-- Name: bib_citation_cache_idx_pubmedid; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_citation_cache_idx_pubmedid ON mgd.bib_citation_cache USING btree (pubmedid);


--
-- Name: bib_citation_cache_idx_relevance_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_citation_cache_idx_relevance_key ON mgd.bib_citation_cache USING btree (_relevance_key);


--
-- Name: bib_notes_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_notes_idx_creation_date ON mgd.bib_notes USING btree (creation_date);


--
-- Name: bib_notes_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_notes_idx_modification_date ON mgd.bib_notes USING btree (modification_date);


--
-- Name: bib_refs_idx_authors; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_refs_idx_authors ON mgd.bib_refs USING btree (((md5(authors))::uuid));


--
-- Name: bib_refs_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_refs_idx_createdby_key ON mgd.bib_refs USING btree (_createdby_key);


--
-- Name: bib_refs_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_refs_idx_creation_date ON mgd.bib_refs USING btree (creation_date);


--
-- Name: bib_refs_idx_isprimary; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_refs_idx_isprimary ON mgd.bib_refs USING btree (_primary);


--
-- Name: bib_refs_idx_journal; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_refs_idx_journal ON mgd.bib_refs USING btree (journal);


--
-- Name: bib_refs_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_refs_idx_modification_date ON mgd.bib_refs USING btree (modification_date);


--
-- Name: bib_refs_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_refs_idx_modifiedby_key ON mgd.bib_refs USING btree (_modifiedby_key);


--
-- Name: bib_refs_idx_referencetype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_refs_idx_referencetype_key ON mgd.bib_refs USING btree (_referencetype_key);


--
-- Name: bib_refs_idx_title; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_refs_idx_title ON mgd.bib_refs USING btree (title);


--
-- Name: bib_refs_idx_year; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_refs_idx_year ON mgd.bib_refs USING btree (year);


--
-- Name: bib_workflow_data_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_data_idx_createdby_key ON mgd.bib_workflow_data USING btree (_createdby_key);


--
-- Name: bib_workflow_data_idx_extractedtext_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_data_idx_extractedtext_key ON mgd.bib_workflow_data USING btree (_extractedtext_key);


--
-- Name: bib_workflow_data_idx_haspdf; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_data_idx_haspdf ON mgd.bib_workflow_data USING btree (haspdf);


--
-- Name: bib_workflow_data_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_data_idx_modifiedby_key ON mgd.bib_workflow_data USING btree (_modifiedby_key);


--
-- Name: bib_workflow_data_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_data_idx_refs_key ON mgd.bib_workflow_data USING btree (_refs_key);


--
-- Name: bib_workflow_data_idx_supplemental_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_data_idx_supplemental_key ON mgd.bib_workflow_data USING btree (_supplemental_key);


--
-- Name: bib_workflow_relevance_idx_confidence; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_relevance_idx_confidence ON mgd.bib_workflow_relevance USING btree (confidence);


--
-- Name: bib_workflow_relevance_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_relevance_idx_createdby_key ON mgd.bib_workflow_relevance USING btree (_createdby_key);


--
-- Name: bib_workflow_relevance_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_relevance_idx_creation_date ON mgd.bib_workflow_relevance USING btree (creation_date);


--
-- Name: bib_workflow_relevance_idx_iscurrent; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_relevance_idx_iscurrent ON mgd.bib_workflow_relevance USING btree (iscurrent);


--
-- Name: bib_workflow_relevance_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_relevance_idx_modification_date ON mgd.bib_workflow_relevance USING btree (modification_date);


--
-- Name: bib_workflow_relevance_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_relevance_idx_modifiedby_key ON mgd.bib_workflow_relevance USING btree (_modifiedby_key);


--
-- Name: bib_workflow_relevance_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_relevance_idx_refs_key ON mgd.bib_workflow_relevance USING btree (_refs_key);


--
-- Name: bib_workflow_relevance_idx_relevance_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_relevance_idx_relevance_key ON mgd.bib_workflow_relevance USING btree (_relevance_key);


--
-- Name: bib_workflow_status_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_status_idx_createdby_key ON mgd.bib_workflow_status USING btree (_createdby_key);


--
-- Name: bib_workflow_status_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_status_idx_creation_date ON mgd.bib_workflow_status USING btree (creation_date);


--
-- Name: bib_workflow_status_idx_group_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_status_idx_group_key ON mgd.bib_workflow_status USING btree (_group_key);


--
-- Name: bib_workflow_status_idx_iscurrent; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_status_idx_iscurrent ON mgd.bib_workflow_status USING btree (iscurrent);


--
-- Name: bib_workflow_status_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_status_idx_modification_date ON mgd.bib_workflow_status USING btree (modification_date);


--
-- Name: bib_workflow_status_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_status_idx_modifiedby_key ON mgd.bib_workflow_status USING btree (_modifiedby_key);


--
-- Name: bib_workflow_status_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_status_idx_refs_key ON mgd.bib_workflow_status USING btree (_refs_key);


--
-- Name: bib_workflow_status_idx_status_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_status_idx_status_key ON mgd.bib_workflow_status USING btree (_status_key);


--
-- Name: bib_workflow_tag_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_tag_idx_createdby_key ON mgd.bib_workflow_tag USING btree (_createdby_key);


--
-- Name: bib_workflow_tag_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_tag_idx_creation_date ON mgd.bib_workflow_tag USING btree (creation_date);


--
-- Name: bib_workflow_tag_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_tag_idx_modification_date ON mgd.bib_workflow_tag USING btree (modification_date);


--
-- Name: bib_workflow_tag_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_tag_idx_modifiedby_key ON mgd.bib_workflow_tag USING btree (_modifiedby_key);


--
-- Name: bib_workflow_tag_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_tag_idx_refs_key ON mgd.bib_workflow_tag USING btree (_refs_key);


--
-- Name: bib_workflow_tag_idx_tag_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX bib_workflow_tag_idx_tag_key ON mgd.bib_workflow_tag USING btree (_tag_key);


--
-- Name: crs_cross_idx_femalestrain_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX crs_cross_idx_femalestrain_key ON mgd.crs_cross USING btree (_femalestrain_key);


--
-- Name: crs_cross_idx_malestrain_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX crs_cross_idx_malestrain_key ON mgd.crs_cross USING btree (_malestrain_key);


--
-- Name: crs_cross_idx_strainho_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX crs_cross_idx_strainho_key ON mgd.crs_cross USING btree (_strainho_key);


--
-- Name: crs_cross_idx_strainht_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX crs_cross_idx_strainht_key ON mgd.crs_cross USING btree (_strainht_key);


--
-- Name: crs_cross_idx_type; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX crs_cross_idx_type ON mgd.crs_cross USING btree (type);


--
-- Name: crs_cross_idx_whosecross; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX crs_cross_idx_whosecross ON mgd.crs_cross USING btree (whosecross);


--
-- Name: crs_matrix_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE UNIQUE INDEX crs_matrix_idx_clustered ON mgd.crs_matrix USING btree (_cross_key, _marker_key, othersymbol, chromosome, rownumber);


--
-- Name: crs_matrix_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX crs_matrix_idx_marker_key ON mgd.crs_matrix USING btree (_marker_key);


--
-- Name: crs_references_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX crs_references_idx_marker_key ON mgd.crs_references USING btree (_marker_key);


--
-- Name: dag_closure_idx_ancestor_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX dag_closure_idx_ancestor_key ON mgd.dag_closure USING btree (_ancestor_key);


--
-- Name: dag_closure_idx_ancestorlabel_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX dag_closure_idx_ancestorlabel_key ON mgd.dag_closure USING btree (_ancestorlabel_key);


--
-- Name: dag_closure_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX dag_closure_idx_clustered ON mgd.dag_closure USING btree (_ancestorobject_key, _descendentobject_key, _dag_key);


--
-- Name: dag_closure_idx_descendent_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX dag_closure_idx_descendent_key ON mgd.dag_closure USING btree (_descendent_key);


--
-- Name: dag_closure_idx_descendentlabel_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX dag_closure_idx_descendentlabel_key ON mgd.dag_closure USING btree (_descendentlabel_key);


--
-- Name: dag_closure_idx_descendentobject_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX dag_closure_idx_descendentobject_key ON mgd.dag_closure USING btree (_descendentobject_key, _ancestorobject_key, _dag_key);


--
-- Name: dag_closure_idx_mgitype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX dag_closure_idx_mgitype_key ON mgd.dag_closure USING btree (_mgitype_key);


--
-- Name: dag_dag_idx_mgitype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX dag_dag_idx_mgitype_key ON mgd.dag_dag USING btree (_mgitype_key);


--
-- Name: dag_dag_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX dag_dag_idx_refs_key ON mgd.dag_dag USING btree (_refs_key);


--
-- Name: dag_edge_idx_child_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX dag_edge_idx_child_key ON mgd.dag_edge USING btree (_child_key);


--
-- Name: dag_edge_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX dag_edge_idx_clustered ON mgd.dag_edge USING btree (_parent_key, _child_key, _label_key);


--
-- Name: dag_edge_idx_dag_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX dag_edge_idx_dag_key ON mgd.dag_edge USING btree (_dag_key);


--
-- Name: dag_edge_idx_label_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX dag_edge_idx_label_key ON mgd.dag_edge USING btree (_label_key);


--
-- Name: dag_label_idx_label; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX dag_label_idx_label ON mgd.dag_label USING btree (label);


--
-- Name: dag_node_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX dag_node_idx_clustered ON mgd.dag_node USING btree (_dag_key, _object_key);


--
-- Name: dag_node_idx_label_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX dag_node_idx_label_key ON mgd.dag_node USING btree (_label_key);


--
-- Name: dag_node_idx_object_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX dag_node_idx_object_key ON mgd.dag_node USING btree (_object_key);


--
-- Name: go_tracking_idx_completedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX go_tracking_idx_completedby_key ON mgd.go_tracking USING btree (_completedby_key);


--
-- Name: go_tracking_idx_completion_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX go_tracking_idx_completion_date ON mgd.go_tracking USING btree (completion_date);


--
-- Name: go_tracking_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX go_tracking_idx_createdby_key ON mgd.go_tracking USING btree (_createdby_key);


--
-- Name: go_tracking_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX go_tracking_idx_creation_date ON mgd.go_tracking USING btree (creation_date);


--
-- Name: go_tracking_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX go_tracking_idx_modification_date ON mgd.go_tracking USING btree (modification_date);


--
-- Name: go_tracking_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX go_tracking_idx_modifiedby_key ON mgd.go_tracking USING btree (_modifiedby_key);


--
-- Name: gxd_allelegenotype_idx_allele_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_allelegenotype_idx_allele_key ON mgd.gxd_allelegenotype USING btree (_allele_key, _genotype_key);


--
-- Name: gxd_allelegenotype_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_allelegenotype_idx_createdby_key ON mgd.gxd_allelegenotype USING btree (_createdby_key);


--
-- Name: gxd_allelegenotype_idx_genotype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_allelegenotype_idx_genotype_key ON mgd.gxd_allelegenotype USING btree (_genotype_key);


--
-- Name: gxd_allelegenotype_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_allelegenotype_idx_marker_key ON mgd.gxd_allelegenotype USING btree (_marker_key);


--
-- Name: gxd_allelegenotype_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_allelegenotype_idx_modifiedby_key ON mgd.gxd_allelegenotype USING btree (_modifiedby_key);


--
-- Name: gxd_allelepair_idx_allele_key_2; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_allelepair_idx_allele_key_2 ON mgd.gxd_allelepair USING btree (_allele_key_2);


--
-- Name: gxd_allelepair_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_allelepair_idx_clustered ON mgd.gxd_allelepair USING btree (_allele_key_1);


--
-- Name: gxd_allelepair_idx_compound_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_allelepair_idx_compound_key ON mgd.gxd_allelepair USING btree (_compound_key);


--
-- Name: gxd_allelepair_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_allelepair_idx_createdby_key ON mgd.gxd_allelepair USING btree (_createdby_key);


--
-- Name: gxd_allelepair_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_allelepair_idx_creation_date ON mgd.gxd_allelepair USING btree (creation_date);


--
-- Name: gxd_allelepair_idx_genotype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_allelepair_idx_genotype_key ON mgd.gxd_allelepair USING btree (_genotype_key);


--
-- Name: gxd_allelepair_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_allelepair_idx_marker_key ON mgd.gxd_allelepair USING btree (_marker_key);


--
-- Name: gxd_allelepair_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_allelepair_idx_modification_date ON mgd.gxd_allelepair USING btree (modification_date);


--
-- Name: gxd_allelepair_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_allelepair_idx_modifiedby_key ON mgd.gxd_allelepair USING btree (_modifiedby_key);


--
-- Name: gxd_allelepair_idx_mutantcellline_key_1; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_allelepair_idx_mutantcellline_key_1 ON mgd.gxd_allelepair USING btree (_mutantcellline_key_1);


--
-- Name: gxd_allelepair_idx_mutantcellline_key_2; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_allelepair_idx_mutantcellline_key_2 ON mgd.gxd_allelepair USING btree (_mutantcellline_key_2);


--
-- Name: gxd_allelepair_idx_pairstate_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_allelepair_idx_pairstate_key ON mgd.gxd_allelepair USING btree (_pairstate_key);


--
-- Name: gxd_antibody_idx_antibodyclass_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_antibody_idx_antibodyclass_key ON mgd.gxd_antibody USING btree (_antibodyclass_key);


--
-- Name: gxd_antibody_idx_antibodytype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_antibody_idx_antibodytype_key ON mgd.gxd_antibody USING btree (_antibodytype_key);


--
-- Name: gxd_antibody_idx_antigen_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_antibody_idx_antigen_key ON mgd.gxd_antibody USING btree (_antigen_key);


--
-- Name: gxd_antibody_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_antibody_idx_createdby_key ON mgd.gxd_antibody USING btree (_createdby_key);


--
-- Name: gxd_antibody_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_antibody_idx_creation_date ON mgd.gxd_antibody USING btree (creation_date);


--
-- Name: gxd_antibody_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_antibody_idx_modification_date ON mgd.gxd_antibody USING btree (modification_date);


--
-- Name: gxd_antibody_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_antibody_idx_modifiedby_key ON mgd.gxd_antibody USING btree (_modifiedby_key);


--
-- Name: gxd_antibody_idx_organism_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_antibody_idx_organism_key ON mgd.gxd_antibody USING btree (_organism_key);


--
-- Name: gxd_antibodyalias_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_antibodyalias_idx_clustered ON mgd.gxd_antibodyalias USING btree (_antibody_key);


--
-- Name: gxd_antibodyalias_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_antibodyalias_idx_refs_key ON mgd.gxd_antibodyalias USING btree (_refs_key);


--
-- Name: gxd_antibodymarker_idx_antibody_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_antibodymarker_idx_antibody_key ON mgd.gxd_antibodymarker USING btree (_antibody_key);


--
-- Name: gxd_antibodymarker_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_antibodymarker_idx_marker_key ON mgd.gxd_antibodymarker USING btree (_marker_key);


--
-- Name: gxd_antibodyprep_idx_antibody_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_antibodyprep_idx_antibody_key ON mgd.gxd_antibodyprep USING btree (_antibody_key);


--
-- Name: gxd_antibodyprep_idx_label_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_antibodyprep_idx_label_key ON mgd.gxd_antibodyprep USING btree (_label_key);


--
-- Name: gxd_antibodyprep_idx_secondary_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_antibodyprep_idx_secondary_key ON mgd.gxd_antibodyprep USING btree (_secondary_key);


--
-- Name: gxd_antigen_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_antigen_idx_createdby_key ON mgd.gxd_antigen USING btree (_createdby_key);


--
-- Name: gxd_antigen_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_antigen_idx_creation_date ON mgd.gxd_antigen USING btree (creation_date);


--
-- Name: gxd_antigen_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_antigen_idx_modification_date ON mgd.gxd_antigen USING btree (modification_date);


--
-- Name: gxd_antigen_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_antigen_idx_modifiedby_key ON mgd.gxd_antigen USING btree (_modifiedby_key);


--
-- Name: gxd_antigen_idx_source_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_antigen_idx_source_key ON mgd.gxd_antigen USING btree (_source_key);


--
-- Name: gxd_assay_idx_antibodyprep_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_assay_idx_antibodyprep_key ON mgd.gxd_assay USING btree (_antibodyprep_key);


--
-- Name: gxd_assay_idx_assaytype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_assay_idx_assaytype_key ON mgd.gxd_assay USING btree (_assaytype_key);


--
-- Name: gxd_assay_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_assay_idx_clustered ON mgd.gxd_assay USING btree (_marker_key);


--
-- Name: gxd_assay_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_assay_idx_createdby_key ON mgd.gxd_assay USING btree (_createdby_key);


--
-- Name: gxd_assay_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_assay_idx_creation_date ON mgd.gxd_assay USING btree (creation_date);


--
-- Name: gxd_assay_idx_imagepane_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_assay_idx_imagepane_key ON mgd.gxd_assay USING btree (_imagepane_key);


--
-- Name: gxd_assay_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_assay_idx_modification_date ON mgd.gxd_assay USING btree (modification_date);


--
-- Name: gxd_assay_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_assay_idx_modifiedby_key ON mgd.gxd_assay USING btree (_modifiedby_key);


--
-- Name: gxd_assay_idx_probeprep_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_assay_idx_probeprep_key ON mgd.gxd_assay USING btree (_probeprep_key);


--
-- Name: gxd_assay_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_assay_idx_refs_key ON mgd.gxd_assay USING btree (_refs_key);


--
-- Name: gxd_assay_idx_reportergene_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_assay_idx_reportergene_key ON mgd.gxd_assay USING btree (_reportergene_key);


--
-- Name: gxd_assaynote_idx_assay_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_assaynote_idx_assay_key ON mgd.gxd_assaynote USING btree (_assay_key);


--
-- Name: gxd_assaynote_idx_assaynote; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_assaynote_idx_assaynote ON mgd.gxd_assaynote USING btree (assaynote);


--
-- Name: gxd_expression_idx_assay_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_expression_idx_assay_key ON mgd.gxd_expression USING btree (_assay_key);


--
-- Name: gxd_expression_idx_assaytype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_expression_idx_assaytype_key ON mgd.gxd_expression USING btree (_assaytype_key);


--
-- Name: gxd_expression_idx_celltype_term_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_expression_idx_celltype_term_key ON mgd.gxd_expression USING btree (_celltype_term_key);


--
-- Name: gxd_expression_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_expression_idx_clustered ON mgd.gxd_expression USING btree (_marker_key, _emapa_term_key, _stage_key, isforgxd);


--
-- Name: gxd_expression_idx_emapa_term_etc_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_expression_idx_emapa_term_etc_key ON mgd.gxd_expression USING btree (_emapa_term_key, _stage_key, _expression_key, isforgxd);


--
-- Name: gxd_expression_idx_emapa_term_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_expression_idx_emapa_term_key ON mgd.gxd_expression USING btree (_emapa_term_key);


--
-- Name: gxd_expression_idx_gellane_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_expression_idx_gellane_key ON mgd.gxd_expression USING btree (_gellane_key);


--
-- Name: gxd_expression_idx_genotype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_expression_idx_genotype_key ON mgd.gxd_expression USING btree (_genotype_key);


--
-- Name: gxd_expression_idx_genotypegxd_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_expression_idx_genotypegxd_key ON mgd.gxd_expression USING btree (_genotype_key, isforgxd);


--
-- Name: gxd_expression_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_expression_idx_marker_key ON mgd.gxd_expression USING btree (_marker_key);


--
-- Name: gxd_expression_idx_refs_etc_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_expression_idx_refs_etc_key ON mgd.gxd_expression USING btree (_refs_key, _emapa_term_key, _stage_key, isforgxd);


--
-- Name: gxd_expression_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_expression_idx_refs_key ON mgd.gxd_expression USING btree (_refs_key);


--
-- Name: gxd_expression_idx_specimen_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_expression_idx_specimen_key ON mgd.gxd_expression USING btree (_specimen_key);


--
-- Name: gxd_expression_idx_stage_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_expression_idx_stage_key ON mgd.gxd_expression USING btree (_stage_key);


--
-- Name: gxd_gelband_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_gelband_idx_clustered ON mgd.gxd_gelband USING btree (_gellane_key);


--
-- Name: gxd_gelband_idx_gelrow_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_gelband_idx_gelrow_key ON mgd.gxd_gelband USING btree (_gelrow_key);


--
-- Name: gxd_gelband_idx_strength_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_gelband_idx_strength_key ON mgd.gxd_gelband USING btree (_strength_key);


--
-- Name: gxd_gellane_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_gellane_idx_clustered ON mgd.gxd_gellane USING btree (_assay_key);


--
-- Name: gxd_gellane_idx_gelcontrol_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_gellane_idx_gelcontrol_key ON mgd.gxd_gellane USING btree (_gelcontrol_key);


--
-- Name: gxd_gellane_idx_gelrnatype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_gellane_idx_gelrnatype_key ON mgd.gxd_gellane USING btree (_gelrnatype_key);


--
-- Name: gxd_gellane_idx_genotype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_gellane_idx_genotype_key ON mgd.gxd_gellane USING btree (_genotype_key);


--
-- Name: gxd_gellanestructure_idx_emapa_term_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_gellanestructure_idx_emapa_term_key ON mgd.gxd_gellanestructure USING btree (_emapa_term_key);


--
-- Name: gxd_gellanestructure_idx_gellane_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_gellanestructure_idx_gellane_key ON mgd.gxd_gellanestructure USING btree (_gellane_key);


--
-- Name: gxd_gellanestructure_idx_stage_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_gellanestructure_idx_stage_key ON mgd.gxd_gellanestructure USING btree (_stage_key);


--
-- Name: gxd_gelrow_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_gelrow_idx_clustered ON mgd.gxd_gelrow USING btree (_assay_key);


--
-- Name: gxd_gelrow_idx_gelunits_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_gelrow_idx_gelunits_key ON mgd.gxd_gelrow USING btree (_gelunits_key);


--
-- Name: gxd_genotype_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_genotype_idx_createdby_key ON mgd.gxd_genotype USING btree (_createdby_key);


--
-- Name: gxd_genotype_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_genotype_idx_creation_date ON mgd.gxd_genotype USING btree (creation_date);


--
-- Name: gxd_genotype_idx_existsas_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_genotype_idx_existsas_key ON mgd.gxd_genotype USING btree (_existsas_key);


--
-- Name: gxd_genotype_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_genotype_idx_modification_date ON mgd.gxd_genotype USING btree (modification_date);


--
-- Name: gxd_genotype_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_genotype_idx_modifiedby_key ON mgd.gxd_genotype USING btree (_modifiedby_key);


--
-- Name: gxd_genotype_idx_strain_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_genotype_idx_strain_key ON mgd.gxd_genotype USING btree (_strain_key);


--
-- Name: gxd_htexperiment_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htexperiment_idx_createdby_key ON mgd.gxd_htexperiment USING btree (_createdby_key);


--
-- Name: gxd_htexperiment_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htexperiment_idx_creation_date ON mgd.gxd_htexperiment USING btree (creation_date);


--
-- Name: gxd_htexperiment_idx_curationstate_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htexperiment_idx_curationstate_key ON mgd.gxd_htexperiment USING btree (_curationstate_key);


--
-- Name: gxd_htexperiment_idx_evaluatedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htexperiment_idx_evaluatedby_key ON mgd.gxd_htexperiment USING btree (_evaluatedby_key);


--
-- Name: gxd_htexperiment_idx_evaluationstate_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htexperiment_idx_evaluationstate_key ON mgd.gxd_htexperiment USING btree (_evaluationstate_key);


--
-- Name: gxd_htexperiment_idx_experimenttype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htexperiment_idx_experimenttype_key ON mgd.gxd_htexperiment USING btree (_experimenttype_key);


--
-- Name: gxd_htexperiment_idx_initialcuratedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htexperiment_idx_initialcuratedby_key ON mgd.gxd_htexperiment USING btree (_initialcuratedby_key);


--
-- Name: gxd_htexperiment_idx_lastcuratedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htexperiment_idx_lastcuratedby_key ON mgd.gxd_htexperiment USING btree (_lastcuratedby_key);


--
-- Name: gxd_htexperiment_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htexperiment_idx_modification_date ON mgd.gxd_htexperiment USING btree (modification_date);


--
-- Name: gxd_htexperiment_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htexperiment_idx_modifiedby_key ON mgd.gxd_htexperiment USING btree (_modifiedby_key);


--
-- Name: gxd_htexperiment_idx_source_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htexperiment_idx_source_key ON mgd.gxd_htexperiment USING btree (_source_key);


--
-- Name: gxd_htexperiment_idx_studytype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htexperiment_idx_studytype_key ON mgd.gxd_htexperiment USING btree (_studytype_key);


--
-- Name: gxd_htexperimentvariable_idx_experiment_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htexperimentvariable_idx_experiment_key ON mgd.gxd_htexperimentvariable USING btree (_experiment_key);


--
-- Name: gxd_htexperimentvariable_idx_term_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htexperimentvariable_idx_term_key ON mgd.gxd_htexperimentvariable USING btree (_term_key);


--
-- Name: gxd_htrawsample_idx_accid; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htrawsample_idx_accid ON mgd.gxd_htrawsample USING btree (accid);


--
-- Name: gxd_htrawsample_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htrawsample_idx_createdby_key ON mgd.gxd_htrawsample USING btree (_createdby_key);


--
-- Name: gxd_htrawsample_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htrawsample_idx_creation_date ON mgd.gxd_htrawsample USING btree (creation_date);


--
-- Name: gxd_htrawsample_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htrawsample_idx_modification_date ON mgd.gxd_htrawsample USING btree (modification_date);


--
-- Name: gxd_htrawsample_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htrawsample_idx_modifiedby_key ON mgd.gxd_htrawsample USING btree (_modifiedby_key);


--
-- Name: gxd_htrawsample_idx_rawsample_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htrawsample_idx_rawsample_key ON mgd.gxd_htrawsample USING btree (_rawsample_key);


--
-- Name: gxd_htsample_idx_celltype_term_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_idx_celltype_term_key ON mgd.gxd_htsample USING btree (_celltype_term_key);


--
-- Name: gxd_htsample_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_idx_createdby_key ON mgd.gxd_htsample USING btree (_createdby_key);


--
-- Name: gxd_htsample_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_idx_creation_date ON mgd.gxd_htsample USING btree (creation_date);


--
-- Name: gxd_htsample_idx_emapa_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_idx_emapa_key ON mgd.gxd_htsample USING btree (_emapa_key);


--
-- Name: gxd_htsample_idx_experiment_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_idx_experiment_key ON mgd.gxd_htsample USING btree (_experiment_key);


--
-- Name: gxd_htsample_idx_genotype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_idx_genotype_key ON mgd.gxd_htsample USING btree (_genotype_key);


--
-- Name: gxd_htsample_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_idx_modification_date ON mgd.gxd_htsample USING btree (modification_date);


--
-- Name: gxd_htsample_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_idx_modifiedby_key ON mgd.gxd_htsample USING btree (_modifiedby_key);


--
-- Name: gxd_htsample_idx_organism_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_idx_organism_key ON mgd.gxd_htsample USING btree (_organism_key);


--
-- Name: gxd_htsample_idx_relevance_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_idx_relevance_key ON mgd.gxd_htsample USING btree (_relevance_key);


--
-- Name: gxd_htsample_idx_sex_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_idx_sex_key ON mgd.gxd_htsample USING btree (_sex_key);


--
-- Name: gxd_htsample_idx_stage_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_idx_stage_key ON mgd.gxd_htsample USING btree (_stage_key);


--
-- Name: gxd_htsample_rnaseq_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseq_idx_createdby_key ON mgd.gxd_htsample_rnaseq USING btree (_createdby_key);


--
-- Name: gxd_htsample_rnaseq_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseq_idx_marker_key ON mgd.gxd_htsample_rnaseq USING btree (_marker_key);


--
-- Name: gxd_htsample_rnaseq_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseq_idx_modifiedby_key ON mgd.gxd_htsample_rnaseq USING btree (_modifiedby_key);


--
-- Name: gxd_htsample_rnaseq_idx_rnaseqcombined_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseq_idx_rnaseqcombined_key ON mgd.gxd_htsample_rnaseq USING btree (_rnaseqcombined_key);


--
-- Name: gxd_htsample_rnaseq_idx_sample_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseq_idx_sample_key ON mgd.gxd_htsample_rnaseq USING btree (_sample_key);


--
-- Name: gxd_htsample_rnaseqcombined_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseqcombined_idx_createdby_key ON mgd.gxd_htsample_rnaseqcombined USING btree (_createdby_key);


--
-- Name: gxd_htsample_rnaseqcombined_idx_level_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseqcombined_idx_level_key ON mgd.gxd_htsample_rnaseqcombined USING btree (_level_key);


--
-- Name: gxd_htsample_rnaseqcombined_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseqcombined_idx_marker_key ON mgd.gxd_htsample_rnaseqcombined USING btree (_marker_key);


--
-- Name: gxd_htsample_rnaseqcombined_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseqcombined_idx_modifiedby_key ON mgd.gxd_htsample_rnaseqcombined USING btree (_modifiedby_key);


--
-- Name: gxd_htsample_rnaseqset_cache_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseqset_cache_idx_createdby_key ON mgd.gxd_htsample_rnaseqset_cache USING btree (_createdby_key);


--
-- Name: gxd_htsample_rnaseqset_cache_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseqset_cache_idx_modifiedby_key ON mgd.gxd_htsample_rnaseqset_cache USING btree (_modifiedby_key);


--
-- Name: gxd_htsample_rnaseqset_cache_idx_rnaseqcombined_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseqset_cache_idx_rnaseqcombined_key ON mgd.gxd_htsample_rnaseqset_cache USING btree (_rnaseqcombined_key);


--
-- Name: gxd_htsample_rnaseqset_cache_idx_rnaseqset_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseqset_cache_idx_rnaseqset_key ON mgd.gxd_htsample_rnaseqset_cache USING btree (_rnaseqset_key);


--
-- Name: gxd_htsample_rnaseqset_idx_age; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseqset_idx_age ON mgd.gxd_htsample_rnaseqset USING btree (age);


--
-- Name: gxd_htsample_rnaseqset_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseqset_idx_createdby_key ON mgd.gxd_htsample_rnaseqset USING btree (_createdby_key);


--
-- Name: gxd_htsample_rnaseqset_idx_emapa_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseqset_idx_emapa_key ON mgd.gxd_htsample_rnaseqset USING btree (_emapa_key);


--
-- Name: gxd_htsample_rnaseqset_idx_experiment_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseqset_idx_experiment_key ON mgd.gxd_htsample_rnaseqset USING btree (_experiment_key);


--
-- Name: gxd_htsample_rnaseqset_idx_genotype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseqset_idx_genotype_key ON mgd.gxd_htsample_rnaseqset USING btree (_genotype_key);


--
-- Name: gxd_htsample_rnaseqset_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseqset_idx_modifiedby_key ON mgd.gxd_htsample_rnaseqset USING btree (_modifiedby_key);


--
-- Name: gxd_htsample_rnaseqset_idx_note; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseqset_idx_note ON mgd.gxd_htsample_rnaseqset USING btree (note);


--
-- Name: gxd_htsample_rnaseqset_idx_organism_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseqset_idx_organism_key ON mgd.gxd_htsample_rnaseqset USING btree (_organism_key);


--
-- Name: gxd_htsample_rnaseqset_idx_sex_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseqset_idx_sex_key ON mgd.gxd_htsample_rnaseqset USING btree (_sex_key);


--
-- Name: gxd_htsample_rnaseqset_idx_stage_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseqset_idx_stage_key ON mgd.gxd_htsample_rnaseqset USING btree (_stage_key);


--
-- Name: gxd_htsample_rnaseqsetmember_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseqsetmember_idx_createdby_key ON mgd.gxd_htsample_rnaseqsetmember USING btree (_createdby_key);


--
-- Name: gxd_htsample_rnaseqsetmember_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseqsetmember_idx_modifiedby_key ON mgd.gxd_htsample_rnaseqsetmember USING btree (_modifiedby_key);


--
-- Name: gxd_htsample_rnaseqsetmember_idx_rnaseqset_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseqsetmember_idx_rnaseqset_key ON mgd.gxd_htsample_rnaseqsetmember USING btree (_rnaseqset_key);


--
-- Name: gxd_htsample_rnaseqsetmember_idx_sample_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_htsample_rnaseqsetmember_idx_sample_key ON mgd.gxd_htsample_rnaseqsetmember USING btree (_sample_key);


--
-- Name: gxd_index_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE UNIQUE INDEX gxd_index_idx_clustered ON mgd.gxd_index USING btree (_marker_key, _refs_key);


--
-- Name: gxd_index_idx_conditionalmutants_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_index_idx_conditionalmutants_key ON mgd.gxd_index USING btree (_conditionalmutants_key);


--
-- Name: gxd_index_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_index_idx_createdby_key ON mgd.gxd_index USING btree (_createdby_key);


--
-- Name: gxd_index_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_index_idx_creation_date ON mgd.gxd_index USING btree (creation_date);


--
-- Name: gxd_index_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_index_idx_modification_date ON mgd.gxd_index USING btree (modification_date);


--
-- Name: gxd_index_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_index_idx_modifiedby_key ON mgd.gxd_index USING btree (_modifiedby_key);


--
-- Name: gxd_index_idx_priority_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_index_idx_priority_key ON mgd.gxd_index USING btree (_priority_key);


--
-- Name: gxd_index_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_index_idx_refs_key ON mgd.gxd_index USING btree (_refs_key);


--
-- Name: gxd_index_stages_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_index_stages_idx_createdby_key ON mgd.gxd_index_stages USING btree (_createdby_key);


--
-- Name: gxd_index_stages_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_index_stages_idx_creation_date ON mgd.gxd_index_stages USING btree (creation_date);


--
-- Name: gxd_index_stages_idx_index_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_index_stages_idx_index_key ON mgd.gxd_index_stages USING btree (_index_key);


--
-- Name: gxd_index_stages_idx_indexassay_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_index_stages_idx_indexassay_key ON mgd.gxd_index_stages USING btree (_indexassay_key);


--
-- Name: gxd_index_stages_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_index_stages_idx_modification_date ON mgd.gxd_index_stages USING btree (modification_date);


--
-- Name: gxd_index_stages_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_index_stages_idx_modifiedby_key ON mgd.gxd_index_stages USING btree (_modifiedby_key);


--
-- Name: gxd_index_stages_idx_stageid_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_index_stages_idx_stageid_key ON mgd.gxd_index_stages USING btree (_stageid_key);


--
-- Name: gxd_insituresult_idx_pattern_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_insituresult_idx_pattern_key ON mgd.gxd_insituresult USING btree (_pattern_key);


--
-- Name: gxd_insituresult_idx_specimen_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_insituresult_idx_specimen_key ON mgd.gxd_insituresult USING btree (_specimen_key);


--
-- Name: gxd_insituresult_idx_strength_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_insituresult_idx_strength_key ON mgd.gxd_insituresult USING btree (_strength_key);


--
-- Name: gxd_insituresultimage_idx_imagepane_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_insituresultimage_idx_imagepane_key ON mgd.gxd_insituresultimage USING btree (_imagepane_key);


--
-- Name: gxd_insituresultimage_idx_result_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_insituresultimage_idx_result_key ON mgd.gxd_insituresultimage USING btree (_result_key);


--
-- Name: gxd_isresultcelltype_idx_celltype_term_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_isresultcelltype_idx_celltype_term_key ON mgd.gxd_isresultcelltype USING btree (_celltype_term_key);


--
-- Name: gxd_isresultcelltype_idx_result_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_isresultcelltype_idx_result_key ON mgd.gxd_isresultcelltype USING btree (_result_key);


--
-- Name: gxd_isresultstructure_idx_emapa_term_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_isresultstructure_idx_emapa_term_key ON mgd.gxd_isresultstructure USING btree (_emapa_term_key);


--
-- Name: gxd_isresultstructure_idx_result_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_isresultstructure_idx_result_key ON mgd.gxd_isresultstructure USING btree (_result_key);


--
-- Name: gxd_isresultstructure_idx_stage_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_isresultstructure_idx_stage_key ON mgd.gxd_isresultstructure USING btree (_stage_key);


--
-- Name: gxd_probeprep_idx_label_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_probeprep_idx_label_key ON mgd.gxd_probeprep USING btree (_label_key);


--
-- Name: gxd_probeprep_idx_probe_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_probeprep_idx_probe_key ON mgd.gxd_probeprep USING btree (_probe_key);


--
-- Name: gxd_probeprep_idx_sense_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_probeprep_idx_sense_key ON mgd.gxd_probeprep USING btree (_sense_key);


--
-- Name: gxd_probeprep_idx_visualization_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_probeprep_idx_visualization_key ON mgd.gxd_probeprep USING btree (_visualization_key);


--
-- Name: gxd_specimen_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_specimen_idx_clustered ON mgd.gxd_specimen USING btree (_assay_key);


--
-- Name: gxd_specimen_idx_embedding_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_specimen_idx_embedding_key ON mgd.gxd_specimen USING btree (_embedding_key);


--
-- Name: gxd_specimen_idx_fixation_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_specimen_idx_fixation_key ON mgd.gxd_specimen USING btree (_fixation_key);


--
-- Name: gxd_specimen_idx_genotype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX gxd_specimen_idx_genotype_key ON mgd.gxd_specimen USING btree (_genotype_key);


--
-- Name: img_image_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX img_image_idx_createdby_key ON mgd.img_image USING btree (_createdby_key);


--
-- Name: img_image_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX img_image_idx_creation_date ON mgd.img_image USING btree (creation_date);


--
-- Name: img_image_idx_imageclass_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX img_image_idx_imageclass_key ON mgd.img_image USING btree (_imageclass_key);


--
-- Name: img_image_idx_imagetype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX img_image_idx_imagetype_key ON mgd.img_image USING btree (_imagetype_key);


--
-- Name: img_image_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX img_image_idx_modification_date ON mgd.img_image USING btree (modification_date);


--
-- Name: img_image_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX img_image_idx_modifiedby_key ON mgd.img_image USING btree (_modifiedby_key);


--
-- Name: img_image_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX img_image_idx_refs_key ON mgd.img_image USING btree (_refs_key);


--
-- Name: img_image_idx_thumbnailimage_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX img_image_idx_thumbnailimage_key ON mgd.img_image USING btree (_thumbnailimage_key);


--
-- Name: img_imagepane_assoc_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX img_imagepane_assoc_idx_createdby_key ON mgd.img_imagepane_assoc USING btree (_createdby_key);


--
-- Name: img_imagepane_assoc_idx_imagepane_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX img_imagepane_assoc_idx_imagepane_key ON mgd.img_imagepane_assoc USING btree (_imagepane_key);


--
-- Name: img_imagepane_assoc_idx_mgitype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX img_imagepane_assoc_idx_mgitype_key ON mgd.img_imagepane_assoc USING btree (_mgitype_key);


--
-- Name: img_imagepane_assoc_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX img_imagepane_assoc_idx_modifiedby_key ON mgd.img_imagepane_assoc USING btree (_modifiedby_key);


--
-- Name: img_imagepane_assoc_idx_object_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX img_imagepane_assoc_idx_object_key ON mgd.img_imagepane_assoc USING btree (_object_key);


--
-- Name: img_imagepane_idx_image_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX img_imagepane_idx_image_key ON mgd.img_imagepane USING btree (_image_key);


--
-- Name: map_coord_collection_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX map_coord_collection_idx_createdby_key ON mgd.map_coord_collection USING btree (_createdby_key);


--
-- Name: map_coord_collection_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX map_coord_collection_idx_modifiedby_key ON mgd.map_coord_collection USING btree (_modifiedby_key);


--
-- Name: map_coord_collection_idx_name; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX map_coord_collection_idx_name ON mgd.map_coord_collection USING btree (name);


--
-- Name: map_coord_feature_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX map_coord_feature_idx_createdby_key ON mgd.map_coord_feature USING btree (_createdby_key);


--
-- Name: map_coord_feature_idx_map_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX map_coord_feature_idx_map_key ON mgd.map_coord_feature USING btree (_map_key);


--
-- Name: map_coord_feature_idx_mgitype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX map_coord_feature_idx_mgitype_key ON mgd.map_coord_feature USING btree (_mgitype_key);


--
-- Name: map_coord_feature_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX map_coord_feature_idx_modifiedby_key ON mgd.map_coord_feature USING btree (_modifiedby_key);


--
-- Name: map_coord_feature_idx_object_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX map_coord_feature_idx_object_key ON mgd.map_coord_feature USING btree (_object_key);


--
-- Name: map_coordinate_idx_collection_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX map_coordinate_idx_collection_key ON mgd.map_coordinate USING btree (_collection_key);


--
-- Name: map_coordinate_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX map_coordinate_idx_createdby_key ON mgd.map_coordinate USING btree (_createdby_key);


--
-- Name: map_coordinate_idx_maptype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX map_coordinate_idx_maptype_key ON mgd.map_coordinate USING btree (_maptype_key);


--
-- Name: map_coordinate_idx_mgitype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX map_coordinate_idx_mgitype_key ON mgd.map_coordinate USING btree (_mgitype_key);


--
-- Name: map_coordinate_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX map_coordinate_idx_modifiedby_key ON mgd.map_coordinate USING btree (_modifiedby_key);


--
-- Name: map_coordinate_idx_object_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX map_coordinate_idx_object_key ON mgd.map_coordinate USING btree (_object_key);


--
-- Name: map_coordinate_idx_units_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX map_coordinate_idx_units_key ON mgd.map_coordinate USING btree (_units_key);


--
-- Name: mgi_keyvalue_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_keyvalue_idx_createdby_key ON mgd.mgi_keyvalue USING btree (_createdby_key);


--
-- Name: mgi_keyvalue_idx_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_keyvalue_idx_key ON mgd.mgi_keyvalue USING btree (key);


--
-- Name: mgi_keyvalue_idx_mgitype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_keyvalue_idx_mgitype_key ON mgd.mgi_keyvalue USING btree (_mgitype_key);


--
-- Name: mgi_keyvalue_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_keyvalue_idx_modifiedby_key ON mgd.mgi_keyvalue USING btree (_modifiedby_key);


--
-- Name: mgi_keyvalue_idx_object_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_keyvalue_idx_object_key ON mgd.mgi_keyvalue USING btree (_object_key);


--
-- Name: mgi_keyvalue_idx_objectkeysequencenum; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_keyvalue_idx_objectkeysequencenum ON mgd.mgi_keyvalue USING btree (_object_key, sequencenum);


--
-- Name: mgi_note_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_note_idx_clustered ON mgd.mgi_note USING btree (_object_key, _mgitype_key, _notetype_key);


--
-- Name: mgi_note_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_note_idx_createdby_key ON mgd.mgi_note USING btree (_createdby_key);


--
-- Name: mgi_note_idx_mgitype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_note_idx_mgitype_key ON mgd.mgi_note USING btree (_mgitype_key);


--
-- Name: mgi_note_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_note_idx_modifiedby_key ON mgd.mgi_note USING btree (_modifiedby_key);


--
-- Name: mgi_note_idx_notetype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_note_idx_notetype_key ON mgd.mgi_note USING btree (_notetype_key);


--
-- Name: mgi_notetype_0; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_notetype_0 ON mgd.mgi_notetype USING btree (lower(notetype));


--
-- Name: mgi_notetype_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_notetype_idx_createdby_key ON mgd.mgi_notetype USING btree (_createdby_key);


--
-- Name: mgi_notetype_idx_mgitype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_notetype_idx_mgitype_key ON mgd.mgi_notetype USING btree (_mgitype_key);


--
-- Name: mgi_notetype_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_notetype_idx_modifiedby_key ON mgd.mgi_notetype USING btree (_modifiedby_key);


--
-- Name: mgi_organism_idx_commonname; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_organism_idx_commonname ON mgd.mgi_organism USING btree (commonname);


--
-- Name: mgi_organism_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_organism_idx_createdby_key ON mgd.mgi_organism USING btree (_createdby_key);


--
-- Name: mgi_organism_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_organism_idx_modifiedby_key ON mgd.mgi_organism USING btree (_modifiedby_key);


--
-- Name: mgi_organism_mgitype_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_organism_mgitype_idx_createdby_key ON mgd.mgi_organism_mgitype USING btree (_createdby_key);


--
-- Name: mgi_organism_mgitype_idx_mgitype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_organism_mgitype_idx_mgitype_key ON mgd.mgi_organism_mgitype USING btree (_mgitype_key);


--
-- Name: mgi_organism_mgitype_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_organism_mgitype_idx_modifiedby_key ON mgd.mgi_organism_mgitype USING btree (_modifiedby_key);


--
-- Name: mgi_organism_mgitype_idx_organism_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_organism_mgitype_idx_organism_key ON mgd.mgi_organism_mgitype USING btree (_organism_key);


--
-- Name: mgi_property_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_property_idx_createdby_key ON mgd.mgi_property USING btree (_createdby_key);


--
-- Name: mgi_property_idx_mgitype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_property_idx_mgitype_key ON mgd.mgi_property USING btree (_mgitype_key);


--
-- Name: mgi_property_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_property_idx_modifiedby_key ON mgd.mgi_property USING btree (_modifiedby_key);


--
-- Name: mgi_property_idx_object_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_property_idx_object_key ON mgd.mgi_property USING btree (_object_key);


--
-- Name: mgi_property_idx_propertyterm_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_property_idx_propertyterm_key ON mgd.mgi_property USING btree (_propertyterm_key);


--
-- Name: mgi_property_idx_propertytype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_property_idx_propertytype_key ON mgd.mgi_property USING btree (_propertytype_key);


--
-- Name: mgi_property_idx_value; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_property_idx_value ON mgd.mgi_property USING btree (value);


--
-- Name: mgi_propertytype_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_propertytype_idx_createdby_key ON mgd.mgi_propertytype USING btree (_createdby_key);


--
-- Name: mgi_propertytype_idx_mgitype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_propertytype_idx_mgitype_key ON mgd.mgi_propertytype USING btree (_mgitype_key);


--
-- Name: mgi_propertytype_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_propertytype_idx_modifiedby_key ON mgd.mgi_propertytype USING btree (_modifiedby_key);


--
-- Name: mgi_propertytype_idx_propertytype; Type: INDEX; Schema: mgd; Owner: -
--

CREATE UNIQUE INDEX mgi_propertytype_idx_propertytype ON mgd.mgi_propertytype USING btree (propertytype);


--
-- Name: mgi_propertytype_idx_vocab_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_propertytype_idx_vocab_key ON mgd.mgi_propertytype USING btree (_vocab_key);


--
-- Name: mgi_refassoctype_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_refassoctype_idx_createdby_key ON mgd.mgi_refassoctype USING btree (_createdby_key);


--
-- Name: mgi_refassoctype_idx_mgitype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_refassoctype_idx_mgitype_key ON mgd.mgi_refassoctype USING btree (_mgitype_key);


--
-- Name: mgi_refassoctype_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_refassoctype_idx_modifiedby_key ON mgd.mgi_refassoctype USING btree (_modifiedby_key);


--
-- Name: mgi_reference_assoc_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_reference_assoc_idx_createdby_key ON mgd.mgi_reference_assoc USING btree (_createdby_key);


--
-- Name: mgi_reference_assoc_idx_mgitype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_reference_assoc_idx_mgitype_key ON mgd.mgi_reference_assoc USING btree (_mgitype_key);


--
-- Name: mgi_reference_assoc_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_reference_assoc_idx_modifiedby_key ON mgd.mgi_reference_assoc USING btree (_modifiedby_key);


--
-- Name: mgi_reference_assoc_idx_object_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_reference_assoc_idx_object_key ON mgd.mgi_reference_assoc USING btree (_object_key);


--
-- Name: mgi_reference_assoc_idx_refassoctype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_reference_assoc_idx_refassoctype_key ON mgd.mgi_reference_assoc USING btree (_refassoctype_key);


--
-- Name: mgi_reference_assoc_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_reference_assoc_idx_refs_key ON mgd.mgi_reference_assoc USING btree (_refs_key);


--
-- Name: mgi_relationship_category_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_relationship_category_idx_modification_date ON mgd.mgi_relationship_category USING btree (modification_date);


--
-- Name: mgi_relationship_category_idx_name; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_relationship_category_idx_name ON mgd.mgi_relationship_category USING btree (name);


--
-- Name: mgi_relationship_idx_category_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_relationship_idx_category_key ON mgd.mgi_relationship USING btree (_category_key);


--
-- Name: mgi_relationship_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_relationship_idx_clustered ON mgd.mgi_relationship USING btree (_object_key_1, _object_key_2);


--
-- Name: mgi_relationship_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_relationship_idx_createdby_key ON mgd.mgi_relationship USING btree (_createdby_key);


--
-- Name: mgi_relationship_idx_evidence_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_relationship_idx_evidence_key ON mgd.mgi_relationship USING btree (_evidence_key);


--
-- Name: mgi_relationship_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_relationship_idx_modifiedby_key ON mgd.mgi_relationship USING btree (_modifiedby_key);


--
-- Name: mgi_relationship_idx_object_key_2; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_relationship_idx_object_key_2 ON mgd.mgi_relationship USING btree (_object_key_2);


--
-- Name: mgi_relationship_idx_qualifier_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_relationship_idx_qualifier_key ON mgd.mgi_relationship USING btree (_qualifier_key);


--
-- Name: mgi_relationship_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_relationship_idx_refs_key ON mgd.mgi_relationship USING btree (_refs_key);


--
-- Name: mgi_relationship_idx_relationshipterm_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_relationship_idx_relationshipterm_key ON mgd.mgi_relationship USING btree (_relationshipterm_key);


--
-- Name: mgi_relationship_property_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_relationship_property_idx_clustered ON mgd.mgi_relationship_property USING btree (_relationship_key, sequencenum);


--
-- Name: mgi_relationship_property_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_relationship_property_idx_createdby_key ON mgd.mgi_relationship_property USING btree (_createdby_key);


--
-- Name: mgi_relationship_property_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_relationship_property_idx_modifiedby_key ON mgd.mgi_relationship_property USING btree (_modifiedby_key);


--
-- Name: mgi_relationship_property_idx_propertyname_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_relationship_property_idx_propertyname_key ON mgd.mgi_relationship_property USING btree (_propertyname_key);


--
-- Name: mgi_set_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_set_idx_clustered ON mgd.mgi_set USING btree (_mgitype_key, name, _set_key, sequencenum);


--
-- Name: mgi_set_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_set_idx_createdby_key ON mgd.mgi_set USING btree (_createdby_key);


--
-- Name: mgi_set_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_set_idx_modifiedby_key ON mgd.mgi_set USING btree (_modifiedby_key);


--
-- Name: mgi_set_idx_name; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_set_idx_name ON mgd.mgi_set USING btree (name, _mgitype_key, _set_key, sequencenum);


--
-- Name: mgi_setmember_emapa_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_setmember_emapa_idx_createdby_key ON mgd.mgi_setmember_emapa USING btree (_createdby_key);


--
-- Name: mgi_setmember_emapa_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_setmember_emapa_idx_modifiedby_key ON mgd.mgi_setmember_emapa USING btree (_modifiedby_key);


--
-- Name: mgi_setmember_emapa_idx_setmember_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_setmember_emapa_idx_setmember_key ON mgd.mgi_setmember_emapa USING btree (_setmember_key);


--
-- Name: mgi_setmember_emapa_idx_stage_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_setmember_emapa_idx_stage_key ON mgd.mgi_setmember_emapa USING btree (_stage_key);


--
-- Name: mgi_setmember_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_setmember_idx_createdby_key ON mgd.mgi_setmember USING btree (_createdby_key);


--
-- Name: mgi_setmember_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_setmember_idx_modifiedby_key ON mgd.mgi_setmember USING btree (_modifiedby_key);


--
-- Name: mgi_setmember_idx_objectset_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_setmember_idx_objectset_key ON mgd.mgi_setmember USING btree (_object_key, _set_key, sequencenum);


--
-- Name: mgi_setmember_idx_setmember_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE UNIQUE INDEX mgi_setmember_idx_setmember_key ON mgd.mgi_setmember USING btree (_setmember_key);


--
-- Name: mgi_synonym_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_synonym_idx_createdby_key ON mgd.mgi_synonym USING btree (_createdby_key);


--
-- Name: mgi_synonym_idx_mgitype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_synonym_idx_mgitype_key ON mgd.mgi_synonym USING btree (_mgitype_key);


--
-- Name: mgi_synonym_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_synonym_idx_modifiedby_key ON mgd.mgi_synonym USING btree (_modifiedby_key);


--
-- Name: mgi_synonym_idx_object_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_synonym_idx_object_key ON mgd.mgi_synonym USING btree (_object_key);


--
-- Name: mgi_synonym_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_synonym_idx_refs_key ON mgd.mgi_synonym USING btree (_refs_key);


--
-- Name: mgi_synonym_idx_synonym; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_synonym_idx_synonym ON mgd.mgi_synonym USING btree (synonym);


--
-- Name: mgi_synonym_idx_synonymtype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_synonym_idx_synonymtype_key ON mgd.mgi_synonym USING btree (_synonymtype_key);


--
-- Name: mgi_synonymtype_0; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_synonymtype_0 ON mgd.mgi_synonymtype USING btree (lower(synonymtype));


--
-- Name: mgi_synonymtype_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_synonymtype_idx_createdby_key ON mgd.mgi_synonymtype USING btree (_createdby_key);


--
-- Name: mgi_synonymtype_idx_mgitype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_synonymtype_idx_mgitype_key ON mgd.mgi_synonymtype USING btree (_mgitype_key);


--
-- Name: mgi_synonymtype_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_synonymtype_idx_modifiedby_key ON mgd.mgi_synonymtype USING btree (_modifiedby_key);


--
-- Name: mgi_synonymtype_idx_organism_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_synonymtype_idx_organism_key ON mgd.mgi_synonymtype USING btree (_organism_key);


--
-- Name: mgi_translation_idx_badname_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_translation_idx_badname_key ON mgd.mgi_translation USING btree (badname);


--
-- Name: mgi_translation_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_translation_idx_createdby_key ON mgd.mgi_translation USING btree (_createdby_key);


--
-- Name: mgi_translation_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_translation_idx_modifiedby_key ON mgd.mgi_translation USING btree (_modifiedby_key);


--
-- Name: mgi_translation_idx_object_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_translation_idx_object_key ON mgd.mgi_translation USING btree (_object_key);


--
-- Name: mgi_translation_idx_translationtype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_translation_idx_translationtype_key ON mgd.mgi_translation USING btree (_translationtype_key);


--
-- Name: mgi_translationtype_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_translationtype_idx_createdby_key ON mgd.mgi_translationtype USING btree (_createdby_key);


--
-- Name: mgi_translationtype_idx_mgitype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_translationtype_idx_mgitype_key ON mgd.mgi_translationtype USING btree (_mgitype_key);


--
-- Name: mgi_translationtype_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_translationtype_idx_modifiedby_key ON mgd.mgi_translationtype USING btree (_modifiedby_key);


--
-- Name: mgi_translationtype_idx_translationtype; Type: INDEX; Schema: mgd; Owner: -
--

CREATE UNIQUE INDEX mgi_translationtype_idx_translationtype ON mgd.mgi_translationtype USING btree (translationtype);


--
-- Name: mgi_translationtype_idx_vocab_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_translationtype_idx_vocab_key ON mgd.mgi_translationtype USING btree (_vocab_key);


--
-- Name: mgi_user_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_user_idx_createdby_key ON mgd.mgi_user USING btree (_createdby_key);


--
-- Name: mgi_user_idx_group_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_user_idx_group_key ON mgd.mgi_user USING btree (_group_key);


--
-- Name: mgi_user_idx_login; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_user_idx_login ON mgd.mgi_user USING btree (login);


--
-- Name: mgi_user_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_user_idx_modifiedby_key ON mgd.mgi_user USING btree (_modifiedby_key);


--
-- Name: mgi_user_idx_userstatus_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_user_idx_userstatus_key ON mgd.mgi_user USING btree (_userstatus_key);


--
-- Name: mgi_user_idx_usertype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mgi_user_idx_usertype_key ON mgd.mgi_user USING btree (_usertype_key);


--
-- Name: mld_assay_types_idx_description; Type: INDEX; Schema: mgd; Owner: -
--

CREATE UNIQUE INDEX mld_assay_types_idx_description ON mgd.mld_assay_types USING btree (description);


--
-- Name: mld_concordance_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_concordance_idx_marker_key ON mgd.mld_concordance USING btree (_marker_key);


--
-- Name: mld_contig_idx_expt_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_contig_idx_expt_key ON mgd.mld_contig USING btree (_expt_key);


--
-- Name: mld_contig_idx_name; Type: INDEX; Schema: mgd; Owner: -
--

CREATE UNIQUE INDEX mld_contig_idx_name ON mgd.mld_contig USING btree (name);


--
-- Name: mld_contigprobe_idx_probe_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_contigprobe_idx_probe_key ON mgd.mld_contigprobe USING btree (_probe_key);


--
-- Name: mld_expt_marker_idx_allele_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_expt_marker_idx_allele_key ON mgd.mld_expt_marker USING btree (_allele_key);


--
-- Name: mld_expt_marker_idx_assay_type_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_expt_marker_idx_assay_type_key ON mgd.mld_expt_marker USING btree (_assay_type_key);


--
-- Name: mld_expt_marker_idx_expt_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_expt_marker_idx_expt_key ON mgd.mld_expt_marker USING btree (_expt_key);


--
-- Name: mld_expt_marker_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_expt_marker_idx_marker_key ON mgd.mld_expt_marker USING btree (_marker_key);


--
-- Name: mld_expts_idx_chromosome; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_expts_idx_chromosome ON mgd.mld_expts USING btree (chromosome);


--
-- Name: mld_expts_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_expts_idx_creation_date ON mgd.mld_expts USING btree (creation_date);


--
-- Name: mld_expts_idx_expttype; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_expts_idx_expttype ON mgd.mld_expts USING btree (expttype);


--
-- Name: mld_expts_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_expts_idx_modification_date ON mgd.mld_expts USING btree (modification_date);


--
-- Name: mld_expts_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_expts_idx_refs_key ON mgd.mld_expts USING btree (_refs_key);


--
-- Name: mld_fish_idx_strain_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_fish_idx_strain_key ON mgd.mld_fish USING btree (_strain_key);


--
-- Name: mld_hit_idx_probe_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_hit_idx_probe_key ON mgd.mld_hit USING btree (_target_key);


--
-- Name: mld_hit_idx_target_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_hit_idx_target_key ON mgd.mld_hit USING btree (_probe_key);


--
-- Name: mld_insitu_idx_strain_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_insitu_idx_strain_key ON mgd.mld_insitu USING btree (_strain_key);


--
-- Name: mld_matrix_idx_cross_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_matrix_idx_cross_key ON mgd.mld_matrix USING btree (_cross_key);


--
-- Name: mld_mc2point_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_mc2point_idx_marker_key ON mgd.mld_mc2point USING btree (_marker_key_1);


--
-- Name: mld_mc2point_idx_marker_key_2; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_mc2point_idx_marker_key_2 ON mgd.mld_mc2point USING btree (_marker_key_2);


--
-- Name: mld_ri2point_idx_marker_key_1; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_ri2point_idx_marker_key_1 ON mgd.mld_ri2point USING btree (_marker_key_1);


--
-- Name: mld_ri2point_idx_marker_key_2; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_ri2point_idx_marker_key_2 ON mgd.mld_ri2point USING btree (_marker_key_2);


--
-- Name: mld_ri_idx_riset_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_ri_idx_riset_key ON mgd.mld_ri USING btree (_riset_key);


--
-- Name: mld_ridata_idx_expt_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_ridata_idx_expt_key ON mgd.mld_ridata USING btree (_expt_key);


--
-- Name: mld_ridata_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_ridata_idx_marker_key ON mgd.mld_ridata USING btree (_marker_key);


--
-- Name: mld_statistics_idx_marker_key_1; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_statistics_idx_marker_key_1 ON mgd.mld_statistics USING btree (_marker_key_1);


--
-- Name: mld_statistics_idx_marker_key_2; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mld_statistics_idx_marker_key_2 ON mgd.mld_statistics USING btree (_marker_key_2);


--
-- Name: mrk_biotypemapping_idx_biotypeterm_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_biotypemapping_idx_biotypeterm_key ON mgd.mrk_biotypemapping USING btree (_biotypeterm_key);


--
-- Name: mrk_biotypemapping_idx_biotypevocab_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_biotypemapping_idx_biotypevocab_key ON mgd.mrk_biotypemapping USING btree (_biotypevocab_key);


--
-- Name: mrk_biotypemapping_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_biotypemapping_idx_createdby_key ON mgd.mrk_biotypemapping USING btree (_createdby_key);


--
-- Name: mrk_biotypemapping_idx_marker_type_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_biotypemapping_idx_marker_type_key ON mgd.mrk_biotypemapping USING btree (_marker_type_key);


--
-- Name: mrk_biotypemapping_idx_mcvterm_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_biotypemapping_idx_mcvterm_key ON mgd.mrk_biotypemapping USING btree (_mcvterm_key);


--
-- Name: mrk_biotypemapping_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_biotypemapping_idx_modifiedby_key ON mgd.mrk_biotypemapping USING btree (_modifiedby_key);


--
-- Name: mrk_biotypemapping_idx_primarymcvterm_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_biotypemapping_idx_primarymcvterm_key ON mgd.mrk_biotypemapping USING btree (_primarymcvterm_key);


--
-- Name: mrk_chromosome_idx_chromosome; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_chromosome_idx_chromosome ON mgd.mrk_chromosome USING btree (chromosome);


--
-- Name: mrk_chromosome_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE UNIQUE INDEX mrk_chromosome_idx_clustered ON mgd.mrk_chromosome USING btree (_organism_key, chromosome);


--
-- Name: mrk_cluster_idx_clusterid; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_cluster_idx_clusterid ON mgd.mrk_cluster USING btree (clusterid);


--
-- Name: mrk_cluster_idx_clustersource_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_cluster_idx_clustersource_key ON mgd.mrk_cluster USING btree (_clustersource_key);


--
-- Name: mrk_cluster_idx_clustertype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_cluster_idx_clustertype_key ON mgd.mrk_cluster USING btree (_clustertype_key);


--
-- Name: mrk_clustermember_idx_cluster_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_clustermember_idx_cluster_key ON mgd.mrk_clustermember USING btree (_cluster_key);


--
-- Name: mrk_clustermember_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_clustermember_idx_marker_key ON mgd.mrk_clustermember USING btree (_marker_key);


--
-- Name: mrk_current_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_current_idx_marker_key ON mgd.mrk_current USING btree (_marker_key);


--
-- Name: mrk_do_cache_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_do_cache_idx_clustered ON mgd.mrk_do_cache USING btree (_term_key, _marker_key);


--
-- Name: mrk_do_cache_idx_genotype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_do_cache_idx_genotype_key ON mgd.mrk_do_cache USING btree (_genotype_key);


--
-- Name: mrk_do_cache_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_do_cache_idx_marker_key ON mgd.mrk_do_cache USING btree (_marker_key);


--
-- Name: mrk_do_cache_idx_organism_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_do_cache_idx_organism_key ON mgd.mrk_do_cache USING btree (_organism_key);


--
-- Name: mrk_do_cache_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_do_cache_idx_refs_key ON mgd.mrk_do_cache USING btree (_refs_key);


--
-- Name: mrk_do_cache_idx_term_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_do_cache_idx_term_key ON mgd.mrk_do_cache USING btree (_term_key);


--
-- Name: mrk_history_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_history_idx_createdby_key ON mgd.mrk_history USING btree (_createdby_key);


--
-- Name: mrk_history_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_history_idx_creation_date ON mgd.mrk_history USING btree (creation_date);


--
-- Name: mrk_history_idx_event_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_history_idx_event_date ON mgd.mrk_history USING btree (event_date);


--
-- Name: mrk_history_idx_history_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_history_idx_history_key ON mgd.mrk_history USING btree (_history_key);


--
-- Name: mrk_history_idx_marker_event_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_history_idx_marker_event_key ON mgd.mrk_history USING btree (_marker_event_key);


--
-- Name: mrk_history_idx_marker_eventreason_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_history_idx_marker_eventreason_key ON mgd.mrk_history USING btree (_marker_eventreason_key);


--
-- Name: mrk_history_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_history_idx_marker_key ON mgd.mrk_history USING btree (_marker_key);


--
-- Name: mrk_history_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_history_idx_modification_date ON mgd.mrk_history USING btree (modification_date);


--
-- Name: mrk_history_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_history_idx_modifiedby_key ON mgd.mrk_history USING btree (_modifiedby_key);


--
-- Name: mrk_history_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_history_idx_refs_key ON mgd.mrk_history USING btree (_refs_key);


--
-- Name: mrk_label_0; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_label_0 ON mgd.mrk_label USING btree (lower(label));


--
-- Name: mrk_label_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_label_idx_clustered ON mgd.mrk_label USING btree (_marker_key, priority, _orthologorganism_key, labeltype, label);


--
-- Name: mrk_label_idx_label; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_label_idx_label ON mgd.mrk_label USING btree (label, _organism_key, _marker_key);


--
-- Name: mrk_label_idx_label_status_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_label_idx_label_status_key ON mgd.mrk_label USING btree (_label_status_key);


--
-- Name: mrk_label_idx_organism_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_label_idx_organism_key ON mgd.mrk_label USING btree (_organism_key);


--
-- Name: mrk_label_idx_priority; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_label_idx_priority ON mgd.mrk_label USING btree (priority);


--
-- Name: mrk_location_cache_0; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_location_cache_0 ON mgd.mrk_location_cache USING btree (lower(chromosome));


--
-- Name: mrk_location_cache_idx_chromosome_cmoffset; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_location_cache_idx_chromosome_cmoffset ON mgd.mrk_location_cache USING btree (chromosome, cmoffset);


--
-- Name: mrk_location_cache_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_location_cache_idx_clustered ON mgd.mrk_location_cache USING btree (chromosome, startcoordinate, endcoordinate);


--
-- Name: mrk_location_cache_idx_marker_type_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_location_cache_idx_marker_type_key ON mgd.mrk_location_cache USING btree (_marker_type_key);


--
-- Name: mrk_location_cache_idx_organism_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_location_cache_idx_organism_key ON mgd.mrk_location_cache USING btree (_organism_key);


--
-- Name: mrk_marker_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_marker_idx_clustered ON mgd.mrk_marker USING btree (chromosome);


--
-- Name: mrk_marker_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_marker_idx_createdby_key ON mgd.mrk_marker USING btree (_createdby_key);


--
-- Name: mrk_marker_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_marker_idx_creation_date ON mgd.mrk_marker USING btree (creation_date);


--
-- Name: mrk_marker_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE UNIQUE INDEX mrk_marker_idx_marker_key ON mgd.mrk_marker USING btree (_marker_key, _organism_key);


--
-- Name: mrk_marker_idx_marker_status_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_marker_idx_marker_status_key ON mgd.mrk_marker USING btree (_marker_status_key);


--
-- Name: mrk_marker_idx_marker_type_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_marker_idx_marker_type_key ON mgd.mrk_marker USING btree (_marker_type_key, _marker_key);


--
-- Name: mrk_marker_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_marker_idx_modification_date ON mgd.mrk_marker USING btree (modification_date);


--
-- Name: mrk_marker_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_marker_idx_modifiedby_key ON mgd.mrk_marker USING btree (_modifiedby_key);


--
-- Name: mrk_marker_idx_organism_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_marker_idx_organism_key ON mgd.mrk_marker USING btree (_organism_key);


--
-- Name: mrk_marker_idx_symbol; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_marker_idx_symbol ON mgd.mrk_marker USING btree (symbol);


--
-- Name: mrk_mcv_cache_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_mcv_cache_idx_marker_key ON mgd.mrk_mcv_cache USING btree (_marker_key);


--
-- Name: mrk_mcv_cache_idx_term; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_mcv_cache_idx_term ON mgd.mrk_mcv_cache USING btree (term);


--
-- Name: mrk_notes_idx_note; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_notes_idx_note ON mgd.mrk_notes USING btree (note);


--
-- Name: mrk_reference_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_reference_idx_refs_key ON mgd.mrk_reference USING btree (_refs_key);


--
-- Name: mrk_status_idx_status; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_status_idx_status ON mgd.mrk_status USING btree (status);


--
-- Name: mrk_strainmarker_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_strainmarker_idx_createdby_key ON mgd.mrk_strainmarker USING btree (_createdby_key);


--
-- Name: mrk_strainmarker_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_strainmarker_idx_marker_key ON mgd.mrk_strainmarker USING btree (_marker_key);


--
-- Name: mrk_strainmarker_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_strainmarker_idx_modifiedby_key ON mgd.mrk_strainmarker USING btree (_modifiedby_key);


--
-- Name: mrk_strainmarker_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_strainmarker_idx_refs_key ON mgd.mrk_strainmarker USING btree (_refs_key);


--
-- Name: mrk_strainmarker_idx_strain_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_strainmarker_idx_strain_key ON mgd.mrk_strainmarker USING btree (_strain_key);


--
-- Name: mrk_types_idx_name; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX mrk_types_idx_name ON mgd.mrk_types USING btree (name);


--
-- Name: prb_alias_idx_alias; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_alias_idx_alias ON mgd.prb_alias USING btree (alias);


--
-- Name: prb_alias_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_alias_idx_clustered ON mgd.prb_alias USING btree (_reference_key);


--
-- Name: prb_alias_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_alias_idx_createdby_key ON mgd.prb_alias USING btree (_createdby_key);


--
-- Name: prb_alias_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_alias_idx_modifiedby_key ON mgd.prb_alias USING btree (_modifiedby_key);


--
-- Name: prb_allele_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_allele_idx_clustered ON mgd.prb_allele USING btree (_rflv_key);


--
-- Name: prb_allele_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_allele_idx_createdby_key ON mgd.prb_allele USING btree (_createdby_key);


--
-- Name: prb_allele_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_allele_idx_modifiedby_key ON mgd.prb_allele USING btree (_modifiedby_key);


--
-- Name: prb_allele_strain_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE UNIQUE INDEX prb_allele_strain_idx_clustered ON mgd.prb_allele_strain USING btree (_allele_key, _strain_key);


--
-- Name: prb_allele_strain_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_allele_strain_idx_createdby_key ON mgd.prb_allele_strain USING btree (_createdby_key);


--
-- Name: prb_allele_strain_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_allele_strain_idx_modifiedby_key ON mgd.prb_allele_strain USING btree (_modifiedby_key);


--
-- Name: prb_allele_strain_idx_strain_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_allele_strain_idx_strain_key ON mgd.prb_allele_strain USING btree (_strain_key);


--
-- Name: prb_marker_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_marker_idx_createdby_key ON mgd.prb_marker USING btree (_createdby_key);


--
-- Name: prb_marker_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_marker_idx_marker_key ON mgd.prb_marker USING btree (_marker_key);


--
-- Name: prb_marker_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_marker_idx_modifiedby_key ON mgd.prb_marker USING btree (_modifiedby_key);


--
-- Name: prb_marker_idx_probe_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_marker_idx_probe_key ON mgd.prb_marker USING btree (_probe_key);


--
-- Name: prb_marker_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_marker_idx_refs_key ON mgd.prb_marker USING btree (_refs_key);


--
-- Name: prb_marker_idx_relationship; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_marker_idx_relationship ON mgd.prb_marker USING btree (relationship);


--
-- Name: prb_notes_idx_note; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_notes_idx_note ON mgd.prb_notes USING btree (note);


--
-- Name: prb_notes_idx_probe_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_notes_idx_probe_key ON mgd.prb_notes USING btree (_probe_key);


--
-- Name: prb_probe_idx_ampprimer; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_probe_idx_ampprimer ON mgd.prb_probe USING btree (ampprimer);


--
-- Name: prb_probe_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE UNIQUE INDEX prb_probe_idx_clustered ON mgd.prb_probe USING btree (_segmenttype_key, _source_key, _probe_key);


--
-- Name: prb_probe_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_probe_idx_createdby_key ON mgd.prb_probe USING btree (_createdby_key);


--
-- Name: prb_probe_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_probe_idx_creation_date ON mgd.prb_probe USING btree (creation_date);


--
-- Name: prb_probe_idx_derivedfrom; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_probe_idx_derivedfrom ON mgd.prb_probe USING btree (derivedfrom);


--
-- Name: prb_probe_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_probe_idx_modification_date ON mgd.prb_probe USING btree (modification_date);


--
-- Name: prb_probe_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_probe_idx_modifiedby_key ON mgd.prb_probe USING btree (_modifiedby_key);


--
-- Name: prb_probe_idx_name; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_probe_idx_name ON mgd.prb_probe USING btree (name);


--
-- Name: prb_probe_idx_source_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_probe_idx_source_key ON mgd.prb_probe USING btree (_source_key);


--
-- Name: prb_probe_idx_vector_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_probe_idx_vector_key ON mgd.prb_probe USING btree (_vector_key);


--
-- Name: prb_ref_notes_idx_note; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_ref_notes_idx_note ON mgd.prb_ref_notes USING btree (note);


--
-- Name: prb_reference_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_reference_idx_clustered ON mgd.prb_reference USING btree (_probe_key);


--
-- Name: prb_reference_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_reference_idx_createdby_key ON mgd.prb_reference USING btree (_createdby_key);


--
-- Name: prb_reference_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_reference_idx_modifiedby_key ON mgd.prb_reference USING btree (_modifiedby_key);


--
-- Name: prb_reference_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_reference_idx_refs_key ON mgd.prb_reference USING btree (_refs_key);


--
-- Name: prb_rflv_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_rflv_idx_clustered ON mgd.prb_rflv USING btree (_reference_key);


--
-- Name: prb_rflv_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_rflv_idx_createdby_key ON mgd.prb_rflv USING btree (_createdby_key);


--
-- Name: prb_rflv_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_rflv_idx_marker_key ON mgd.prb_rflv USING btree (_marker_key);


--
-- Name: prb_rflv_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_rflv_idx_modifiedby_key ON mgd.prb_rflv USING btree (_modifiedby_key);


--
-- Name: prb_source_idx_cellline_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_source_idx_cellline_key ON mgd.prb_source USING btree (_cellline_key);


--
-- Name: prb_source_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_source_idx_clustered ON mgd.prb_source USING btree (_organism_key, _source_key);


--
-- Name: prb_source_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_source_idx_createdby_key ON mgd.prb_source USING btree (_createdby_key);


--
-- Name: prb_source_idx_gender_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_source_idx_gender_key ON mgd.prb_source USING btree (_gender_key);


--
-- Name: prb_source_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_source_idx_modifiedby_key ON mgd.prb_source USING btree (_modifiedby_key);


--
-- Name: prb_source_idx_name; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_source_idx_name ON mgd.prb_source USING btree (name);


--
-- Name: prb_source_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_source_idx_refs_key ON mgd.prb_source USING btree (_refs_key);


--
-- Name: prb_source_idx_segmenttype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_source_idx_segmenttype_key ON mgd.prb_source USING btree (_segmenttype_key);


--
-- Name: prb_source_idx_strain_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_source_idx_strain_key ON mgd.prb_source USING btree (_strain_key);


--
-- Name: prb_source_idx_tissue_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_source_idx_tissue_key ON mgd.prb_source USING btree (_tissue_key);


--
-- Name: prb_source_idx_vector_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_source_idx_vector_key ON mgd.prb_source USING btree (_vector_key);


--
-- Name: prb_strain_genotype_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_strain_genotype_idx_clustered ON mgd.prb_strain_genotype USING btree (_strain_key);


--
-- Name: prb_strain_genotype_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_strain_genotype_idx_createdby_key ON mgd.prb_strain_genotype USING btree (_createdby_key);


--
-- Name: prb_strain_genotype_idx_genotype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_strain_genotype_idx_genotype_key ON mgd.prb_strain_genotype USING btree (_genotype_key);


--
-- Name: prb_strain_genotype_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_strain_genotype_idx_modifiedby_key ON mgd.prb_strain_genotype USING btree (_modifiedby_key);


--
-- Name: prb_strain_genotype_idx_qualifier_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_strain_genotype_idx_qualifier_key ON mgd.prb_strain_genotype USING btree (_qualifier_key);


--
-- Name: prb_strain_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_strain_idx_createdby_key ON mgd.prb_strain USING btree (_createdby_key);


--
-- Name: prb_strain_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_strain_idx_creation_date ON mgd.prb_strain USING btree (creation_date);


--
-- Name: prb_strain_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_strain_idx_modification_date ON mgd.prb_strain USING btree (modification_date);


--
-- Name: prb_strain_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_strain_idx_modifiedby_key ON mgd.prb_strain USING btree (_modifiedby_key);


--
-- Name: prb_strain_idx_species_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_strain_idx_species_key ON mgd.prb_strain USING btree (_species_key);


--
-- Name: prb_strain_idx_strain; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_strain_idx_strain ON mgd.prb_strain USING btree (strain);


--
-- Name: prb_strain_idx_straintype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_strain_idx_straintype_key ON mgd.prb_strain USING btree (_straintype_key);


--
-- Name: prb_strain_marker_idx_allele_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_strain_marker_idx_allele_key ON mgd.prb_strain_marker USING btree (_allele_key);


--
-- Name: prb_strain_marker_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_strain_marker_idx_clustered ON mgd.prb_strain_marker USING btree (_strain_key);


--
-- Name: prb_strain_marker_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_strain_marker_idx_createdby_key ON mgd.prb_strain_marker USING btree (_createdby_key);


--
-- Name: prb_strain_marker_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_strain_marker_idx_marker_key ON mgd.prb_strain_marker USING btree (_marker_key);


--
-- Name: prb_strain_marker_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_strain_marker_idx_modifiedby_key ON mgd.prb_strain_marker USING btree (_modifiedby_key);


--
-- Name: prb_strain_marker_idx_qualifier_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_strain_marker_idx_qualifier_key ON mgd.prb_strain_marker USING btree (_qualifier_key);


--
-- Name: prb_tissue_idx_tissue; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX prb_tissue_idx_tissue ON mgd.prb_tissue USING btree (tissue);


--
-- Name: ri_riset_idx_designation; Type: INDEX; Schema: mgd; Owner: -
--

CREATE UNIQUE INDEX ri_riset_idx_designation ON mgd.ri_riset USING btree (designation);


--
-- Name: ri_riset_idx_strain_key_1; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX ri_riset_idx_strain_key_1 ON mgd.ri_riset USING btree (_strain_key_1);


--
-- Name: ri_riset_idx_strain_key_2; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX ri_riset_idx_strain_key_2 ON mgd.ri_riset USING btree (_strain_key_2);


--
-- Name: ri_summary_expt_ref_idx_expt_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX ri_summary_expt_ref_idx_expt_key ON mgd.ri_summary_expt_ref USING btree (_expt_key);


--
-- Name: ri_summary_expt_ref_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX ri_summary_expt_ref_idx_refs_key ON mgd.ri_summary_expt_ref USING btree (_refs_key);


--
-- Name: ri_summary_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX ri_summary_idx_marker_key ON mgd.ri_summary USING btree (_marker_key);


--
-- Name: ri_summary_idx_riset_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX ri_summary_idx_riset_key ON mgd.ri_summary USING btree (_riset_key);


--
-- Name: seq_allele_assoc_idx_allele_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_allele_assoc_idx_allele_key ON mgd.seq_allele_assoc USING btree (_allele_key);


--
-- Name: seq_allele_assoc_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_allele_assoc_idx_createdby_key ON mgd.seq_allele_assoc USING btree (_createdby_key);


--
-- Name: seq_allele_assoc_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_allele_assoc_idx_modifiedby_key ON mgd.seq_allele_assoc USING btree (_modifiedby_key);


--
-- Name: seq_allele_assoc_idx_qualifier_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_allele_assoc_idx_qualifier_key ON mgd.seq_allele_assoc USING btree (_qualifier_key);


--
-- Name: seq_allele_assoc_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_allele_assoc_idx_refs_key ON mgd.seq_allele_assoc USING btree (_refs_key);


--
-- Name: seq_allele_assoc_idx_sequence_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_allele_assoc_idx_sequence_key ON mgd.seq_allele_assoc USING btree (_sequence_key, _allele_key);


--
-- Name: seq_coord_cache_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_coord_cache_idx_clustered ON mgd.seq_coord_cache USING btree (chromosome, startcoordinate, endcoordinate);


--
-- Name: seq_coord_cache_idx_sequence_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_coord_cache_idx_sequence_key ON mgd.seq_coord_cache USING btree (_sequence_key);


--
-- Name: seq_genemodel_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_genemodel_idx_createdby_key ON mgd.seq_genemodel USING btree (_createdby_key);


--
-- Name: seq_genemodel_idx_gmmarker_type_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_genemodel_idx_gmmarker_type_key ON mgd.seq_genemodel USING btree (_gmmarker_type_key);


--
-- Name: seq_genemodel_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_genemodel_idx_modifiedby_key ON mgd.seq_genemodel USING btree (_modifiedby_key);


--
-- Name: seq_genetrap_idx_reversecomp_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_genetrap_idx_reversecomp_key ON mgd.seq_genetrap USING btree (_reversecomp_key);


--
-- Name: seq_genetrap_idx_tagmethod_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_genetrap_idx_tagmethod_key ON mgd.seq_genetrap USING btree (_tagmethod_key);


--
-- Name: seq_genetrap_idx_vectorend_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_genetrap_idx_vectorend_key ON mgd.seq_genetrap USING btree (_vectorend_key);


--
-- Name: seq_marker_cache_idx_accid; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_marker_cache_idx_accid ON mgd.seq_marker_cache USING btree (accid);


--
-- Name: seq_marker_cache_idx_annotation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_marker_cache_idx_annotation_date ON mgd.seq_marker_cache USING btree (annotation_date);


--
-- Name: seq_marker_cache_idx_biotypeconflict_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_marker_cache_idx_biotypeconflict_key ON mgd.seq_marker_cache USING btree (_biotypeconflict_key);


--
-- Name: seq_marker_cache_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_marker_cache_idx_clustered ON mgd.seq_marker_cache USING btree (_sequence_key, _marker_key, _refs_key);


--
-- Name: seq_marker_cache_idx_logicaldb_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_marker_cache_idx_logicaldb_key ON mgd.seq_marker_cache USING btree (_logicaldb_key);


--
-- Name: seq_marker_cache_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_marker_cache_idx_marker_key ON mgd.seq_marker_cache USING btree (_marker_key, _sequence_key);


--
-- Name: seq_marker_cache_idx_marker_type_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_marker_cache_idx_marker_type_key ON mgd.seq_marker_cache USING btree (_marker_type_key);


--
-- Name: seq_marker_cache_idx_organism_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_marker_cache_idx_organism_key ON mgd.seq_marker_cache USING btree (_organism_key);


--
-- Name: seq_marker_cache_idx_qualifier_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_marker_cache_idx_qualifier_key ON mgd.seq_marker_cache USING btree (_qualifier_key);


--
-- Name: seq_marker_cache_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_marker_cache_idx_refs_key ON mgd.seq_marker_cache USING btree (_refs_key);


--
-- Name: seq_marker_cache_idx_sequenceprovider_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_marker_cache_idx_sequenceprovider_key ON mgd.seq_marker_cache USING btree (_sequenceprovider_key);


--
-- Name: seq_marker_cache_idx_sequencetype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_marker_cache_idx_sequencetype_key ON mgd.seq_marker_cache USING btree (_sequencetype_key);


--
-- Name: seq_probe_cache_idx_annotation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_probe_cache_idx_annotation_date ON mgd.seq_probe_cache USING btree (annotation_date);


--
-- Name: seq_probe_cache_idx_probe_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_probe_cache_idx_probe_key ON mgd.seq_probe_cache USING btree (_probe_key, _sequence_key);


--
-- Name: seq_probe_cache_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_probe_cache_idx_refs_key ON mgd.seq_probe_cache USING btree (_refs_key);


--
-- Name: seq_sequence_assoc_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_sequence_assoc_idx_clustered ON mgd.seq_sequence_assoc USING btree (_sequence_key_1, _sequence_key_2, _qualifier_key);


--
-- Name: seq_sequence_assoc_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_sequence_assoc_idx_createdby_key ON mgd.seq_sequence_assoc USING btree (_createdby_key);


--
-- Name: seq_sequence_assoc_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_sequence_assoc_idx_modifiedby_key ON mgd.seq_sequence_assoc USING btree (_modifiedby_key);


--
-- Name: seq_sequence_assoc_idx_qualifier_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_sequence_assoc_idx_qualifier_key ON mgd.seq_sequence_assoc USING btree (_qualifier_key);


--
-- Name: seq_sequence_assoc_idx_sequence_key_2; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_sequence_assoc_idx_sequence_key_2 ON mgd.seq_sequence_assoc USING btree (_sequence_key_2, _sequence_key_1, _qualifier_key);


--
-- Name: seq_sequence_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_sequence_idx_clustered ON mgd.seq_sequence USING btree (_sequencetype_key, _sequenceprovider_key, length);


--
-- Name: seq_sequence_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_sequence_idx_createdby_key ON mgd.seq_sequence USING btree (_createdby_key);


--
-- Name: seq_sequence_idx_length; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_sequence_idx_length ON mgd.seq_sequence USING btree (length);


--
-- Name: seq_sequence_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_sequence_idx_modifiedby_key ON mgd.seq_sequence USING btree (_modifiedby_key);


--
-- Name: seq_sequence_idx_seqrecord_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_sequence_idx_seqrecord_date ON mgd.seq_sequence USING btree (seqrecord_date);


--
-- Name: seq_sequence_idx_sequenceprovider_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_sequence_idx_sequenceprovider_key ON mgd.seq_sequence USING btree (_sequenceprovider_key);


--
-- Name: seq_sequence_idx_sequencequality_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_sequence_idx_sequencequality_key ON mgd.seq_sequence USING btree (_sequencequality_key);


--
-- Name: seq_sequence_idx_sequencestatus_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_sequence_idx_sequencestatus_key ON mgd.seq_sequence USING btree (_sequencestatus_key);


--
-- Name: seq_sequence_idx_sequencetype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_sequence_idx_sequencetype_key ON mgd.seq_sequence USING btree (_sequencetype_key);


--
-- Name: seq_sequence_raw_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_sequence_raw_idx_createdby_key ON mgd.seq_sequence_raw USING btree (_createdby_key);


--
-- Name: seq_sequence_raw_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_sequence_raw_idx_modifiedby_key ON mgd.seq_sequence_raw USING btree (_modifiedby_key);


--
-- Name: seq_source_assoc_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE UNIQUE INDEX seq_source_assoc_idx_clustered ON mgd.seq_source_assoc USING btree (_sequence_key, _source_key);


--
-- Name: seq_source_assoc_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_source_assoc_idx_createdby_key ON mgd.seq_source_assoc USING btree (_createdby_key);


--
-- Name: seq_source_assoc_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_source_assoc_idx_modifiedby_key ON mgd.seq_source_assoc USING btree (_modifiedby_key);


--
-- Name: seq_source_assoc_idx_source_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX seq_source_assoc_idx_source_key ON mgd.seq_source_assoc USING btree (_source_key);


--
-- Name: voc_allele_cache_idx_allele_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_allele_cache_idx_allele_key ON mgd.voc_allele_cache USING btree (_allele_key, _term_key);


--
-- Name: voc_allele_cache_idx_annottype; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_allele_cache_idx_annottype ON mgd.voc_allele_cache USING btree (annottype, _allele_key);


--
-- Name: voc_annot_count_cache_idx_annottype; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_annot_count_cache_idx_annottype ON mgd.voc_annot_count_cache USING btree (annottype, _term_key);


--
-- Name: voc_annot_idx_annottype_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_annot_idx_annottype_key ON mgd.voc_annot USING btree (_annottype_key);


--
-- Name: voc_annot_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_annot_idx_clustered ON mgd.voc_annot USING btree (_object_key, _term_key, _annottype_key, _qualifier_key);


--
-- Name: voc_annot_idx_object_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_annot_idx_object_key ON mgd.voc_annot USING btree (_object_key);


--
-- Name: voc_annot_idx_qualifier_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_annot_idx_qualifier_key ON mgd.voc_annot USING btree (_qualifier_key);


--
-- Name: voc_annot_idx_term_etc_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_annot_idx_term_etc_key ON mgd.voc_annot USING btree (_term_key, _annottype_key, _qualifier_key, _object_key);


--
-- Name: voc_annot_idx_term_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_annot_idx_term_key ON mgd.voc_annot USING btree (_term_key);


--
-- Name: voc_annotheader_idx_approvedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_annotheader_idx_approvedby_key ON mgd.voc_annotheader USING btree (_approvedby_key);


--
-- Name: voc_annotheader_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_annotheader_idx_clustered ON mgd.voc_annotheader USING btree (_annottype_key, _object_key, _term_key);


--
-- Name: voc_annotheader_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_annotheader_idx_createdby_key ON mgd.voc_annotheader USING btree (_createdby_key);


--
-- Name: voc_annotheader_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_annotheader_idx_modifiedby_key ON mgd.voc_annotheader USING btree (_modifiedby_key);


--
-- Name: voc_annotheader_idx_object_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_annotheader_idx_object_key ON mgd.voc_annotheader USING btree (_object_key);


--
-- Name: voc_annotheader_idx_term_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_annotheader_idx_term_key ON mgd.voc_annotheader USING btree (_term_key);


--
-- Name: voc_annottype_0; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_annottype_0 ON mgd.voc_annottype USING btree (lower(name));


--
-- Name: voc_annottype_idx_evidencevocab_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_annottype_idx_evidencevocab_key ON mgd.voc_annottype USING btree (_evidencevocab_key);


--
-- Name: voc_annottype_idx_mgitypevocabevidence; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_annottype_idx_mgitypevocabevidence ON mgd.voc_annottype USING btree (_mgitype_key, _vocab_key, _evidencevocab_key);


--
-- Name: voc_annottype_idx_name; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_annottype_idx_name ON mgd.voc_annottype USING btree (name);


--
-- Name: voc_annottype_idx_qualifiervocab_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_annottype_idx_qualifiervocab_key ON mgd.voc_annottype USING btree (_qualifiervocab_key);


--
-- Name: voc_annottype_idx_vocab_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_annottype_idx_vocab_key ON mgd.voc_annottype USING btree (_vocab_key);


--
-- Name: voc_evidence_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_evidence_idx_clustered ON mgd.voc_evidence USING btree (_annot_key);


--
-- Name: voc_evidence_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_evidence_idx_createdby_key ON mgd.voc_evidence USING btree (_createdby_key);


--
-- Name: voc_evidence_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_evidence_idx_creation_date ON mgd.voc_evidence USING btree (creation_date);


--
-- Name: voc_evidence_idx_evidenceterm_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_evidence_idx_evidenceterm_key ON mgd.voc_evidence USING btree (_evidenceterm_key);


--
-- Name: voc_evidence_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_evidence_idx_modification_date ON mgd.voc_evidence USING btree (modification_date);


--
-- Name: voc_evidence_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_evidence_idx_modifiedby_key ON mgd.voc_evidence USING btree (_modifiedby_key);


--
-- Name: voc_evidence_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_evidence_idx_refs_key ON mgd.voc_evidence USING btree (_refs_key);


--
-- Name: voc_evidence_property_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_evidence_property_idx_clustered ON mgd.voc_evidence_property USING btree (_annotevidence_key);


--
-- Name: voc_evidence_property_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_evidence_property_idx_createdby_key ON mgd.voc_evidence_property USING btree (_createdby_key);


--
-- Name: voc_evidence_property_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_evidence_property_idx_modifiedby_key ON mgd.voc_evidence_property USING btree (_modifiedby_key);


--
-- Name: voc_evidence_property_idx_propertyterm_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_evidence_property_idx_propertyterm_key ON mgd.voc_evidence_property USING btree (_propertyterm_key);


--
-- Name: voc_marker_cache_idx_annottype; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_marker_cache_idx_annottype ON mgd.voc_marker_cache USING btree (annottype);


--
-- Name: voc_marker_cache_idx_annottype_etc; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_marker_cache_idx_annottype_etc ON mgd.voc_marker_cache USING btree (annottype, _marker_key);


--
-- Name: voc_marker_cache_idx_marker_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_marker_cache_idx_marker_key ON mgd.voc_marker_cache USING btree (_marker_key, _term_key);


--
-- Name: voc_marker_cache_idx_term_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_marker_cache_idx_term_key ON mgd.voc_marker_cache USING btree (_term_key, _marker_key);


--
-- Name: voc_term_0; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_term_0 ON mgd.voc_term USING btree (lower(term));


--
-- Name: voc_term_emapa_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_term_emapa_idx_createdby_key ON mgd.voc_term_emapa USING btree (_createdby_key);


--
-- Name: voc_term_emapa_idx_endstage; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_term_emapa_idx_endstage ON mgd.voc_term_emapa USING btree (endstage);


--
-- Name: voc_term_emapa_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_term_emapa_idx_modifiedby_key ON mgd.voc_term_emapa USING btree (_modifiedby_key);


--
-- Name: voc_term_emapa_idx_parent; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_term_emapa_idx_parent ON mgd.voc_term_emapa USING btree (_defaultparent_key);


--
-- Name: voc_term_emapa_idx_startstage; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_term_emapa_idx_startstage ON mgd.voc_term_emapa USING btree (startstage);


--
-- Name: voc_term_emaps_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_term_emaps_idx_createdby_key ON mgd.voc_term_emaps USING btree (_createdby_key);


--
-- Name: voc_term_emaps_idx_emapa; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_term_emaps_idx_emapa ON mgd.voc_term_emaps USING btree (_emapa_term_key);


--
-- Name: voc_term_emaps_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_term_emaps_idx_modifiedby_key ON mgd.voc_term_emaps USING btree (_modifiedby_key);


--
-- Name: voc_term_emaps_idx_parent; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_term_emaps_idx_parent ON mgd.voc_term_emaps USING btree (_defaultparent_key);


--
-- Name: voc_term_emaps_idx_stage_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_term_emaps_idx_stage_key ON mgd.voc_term_emaps USING btree (_stage_key);


--
-- Name: voc_term_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_term_idx_clustered ON mgd.voc_term USING btree (_vocab_key, sequencenum, term, _term_key);


--
-- Name: voc_term_idx_createdby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_term_idx_createdby_key ON mgd.voc_term USING btree (_createdby_key);


--
-- Name: voc_term_idx_creation_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_term_idx_creation_date ON mgd.voc_term USING btree (creation_date);


--
-- Name: voc_term_idx_modification_date; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_term_idx_modification_date ON mgd.voc_term USING btree (modification_date);


--
-- Name: voc_term_idx_modifiedby_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_term_idx_modifiedby_key ON mgd.voc_term USING btree (_modifiedby_key);


--
-- Name: voc_term_idx_term; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_term_idx_term ON mgd.voc_term USING btree (term, _term_key, _vocab_key);


--
-- Name: voc_term_idx_vocab_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_term_idx_vocab_key ON mgd.voc_term USING btree (_vocab_key);


--
-- Name: voc_vocab_idx_name; Type: INDEX; Schema: mgd; Owner: -
--

CREATE UNIQUE INDEX voc_vocab_idx_name ON mgd.voc_vocab USING btree (name, _vocab_key);


--
-- Name: voc_vocab_idx_refs_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_vocab_idx_refs_key ON mgd.voc_vocab USING btree (_refs_key);


--
-- Name: voc_vocabdag_idx_dag_key; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX voc_vocabdag_idx_dag_key ON mgd.voc_vocabdag USING btree (_dag_key);


--
-- Name: wks_rosetta_idx_clustered; Type: INDEX; Schema: mgd; Owner: -
--

CREATE INDEX wks_rosetta_idx_clustered ON mgd.wks_rosetta USING btree (_marker_key);


--
-- Name: gxd_index_summaryby_view _RETURN; Type: RULE; Schema: mgd; Owner: -
--

CREATE OR REPLACE VIEW mgd.gxd_index_summaryby_view AS
 SELECT DISTINCT g._index_key,
    a1.accid AS markerid,
    m._marker_key,
    m.symbol,
    m.name,
    s.status AS markerstatus,
    t.name AS markertype,
    gs._indexassay_key,
    t1.term AS indexassay,
    t1.sequencenum,
    t2.term AS age,
    t3.term AS priority,
    t4.term AS conditional,
        CASE
            WHEN (EXISTS ( SELECT 1
               FROM mgd.gxd_expression gx
              WHERE ((gx._assaytype_key <> ALL (ARRAY[10, 11])) AND (gx._refs_key = g._refs_key)))) THEN 1
            ELSE 0
        END AS isfullcoded,
    bc.jnumid,
    bc.short_citation,
    g.comments,
    array_to_string(array_agg(DISTINCT ms.synonym), ', '::text) AS synonyms
   FROM mgd.gxd_index g,
    mgd.gxd_index_stages gs,
    mgd.mrk_marker m,
    mgd.acc_accession a1,
    mgd.mrk_status s,
    mgd.mrk_types t,
    mgd.bib_citation_cache bc,
    mgd.voc_term t1,
    mgd.voc_term t2,
    mgd.voc_term t3,
    mgd.voc_term t4,
    mgd.mgi_synonym ms
  WHERE ((g._marker_key = m._marker_key) AND (g._marker_key = a1._object_key) AND (a1._mgitype_key = 2) AND (a1._logicaldb_key = 1) AND (a1.preferred = 1) AND (m._marker_status_key = s._marker_status_key) AND (m._marker_type_key = t._marker_type_key) AND (g._index_key = gs._index_key) AND (gs._indexassay_key = t1._term_key) AND (gs._stageid_key = t2._term_key) AND (g._refs_key = bc._refs_key) AND (g._priority_key = t3._term_key) AND (g._conditionalmutants_key = t4._term_key) AND (g._marker_key = ms._object_key) AND (ms._mgitype_key = 2))
  GROUP BY g._index_key, a1.accid, m._marker_key, m.symbol, m.name, s.status, t.name, gs._indexassay_key, t1.term, t1.sequencenum, t2.term, t3.term, t4.term,
        CASE
            WHEN (EXISTS ( SELECT 1
               FROM mgd.gxd_expression gx
              WHERE ((gx._assaytype_key <> ALL (ARRAY[10, 11])) AND (gx._refs_key = g._refs_key)))) THEN 1
            ELSE 0
        END, bc.jnumid, bc.short_citation
UNION
 SELECT DISTINCT g._index_key,
    a1.accid AS markerid,
    m._marker_key,
    m.symbol,
    m.name,
    s.status AS markerstatus,
    t.name AS markertype,
    gs._indexassay_key,
    t1.term AS indexassay,
    t1.sequencenum,
    t2.term AS age,
    t3.term AS priority,
    t4.term AS conditional,
        CASE
            WHEN (EXISTS ( SELECT 1
               FROM mgd.gxd_expression gx
              WHERE ((gx._assaytype_key <> ALL (ARRAY[10, 11])) AND (gx._refs_key = g._refs_key)))) THEN 1
            ELSE 0
        END AS isfullcoded,
    bc.jnumid,
    bc.short_citation,
    g.comments,
    NULL::text AS synonyms
   FROM mgd.gxd_index g,
    mgd.gxd_index_stages gs,
    mgd.mrk_marker m,
    mgd.acc_accession a1,
    mgd.mrk_status s,
    mgd.mrk_types t,
    mgd.bib_citation_cache bc,
    mgd.voc_term t1,
    mgd.voc_term t2,
    mgd.voc_term t3,
    mgd.voc_term t4
  WHERE ((g._marker_key = m._marker_key) AND (g._marker_key = a1._object_key) AND (a1._mgitype_key = 2) AND (a1._logicaldb_key = 1) AND (a1.preferred = 1) AND (m._marker_status_key = s._marker_status_key) AND (m._marker_type_key = t._marker_type_key) AND (g._index_key = gs._index_key) AND (gs._indexassay_key = t1._term_key) AND (gs._stageid_key = t2._term_key) AND (g._refs_key = bc._refs_key) AND (g._priority_key = t3._term_key) AND (g._conditionalmutants_key = t4._term_key) AND (NOT (EXISTS ( SELECT 1
           FROM mgd.mgi_synonym ms
          WHERE ((g._marker_key = ms._object_key) AND (ms._mgitype_key = 2))))))
  GROUP BY g._index_key, a1.accid, m._marker_key, m.symbol, m.name, s.status, t.name, gs._indexassay_key, t1.term, t1.sequencenum, t2.term, t3.term, t4.term,
        CASE
            WHEN (EXISTS ( SELECT 1
               FROM mgd.gxd_expression gx
              WHERE ((gx._assaytype_key <> ALL (ARRAY[10, 11])) AND (gx._refs_key = g._refs_key)))) THEN 1
            ELSE 0
        END, bc.jnumid, bc.short_citation
  ORDER BY 4, 16, 10;


--
-- Name: acc_accession acc_accession_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER acc_accession_delete_trigger AFTER DELETE ON mgd.acc_accession FOR EACH ROW EXECUTE FUNCTION mgd.acc_accession_delete();


--
-- Name: all_allele all_allele_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER all_allele_delete_trigger AFTER DELETE ON mgd.all_allele FOR EACH ROW EXECUTE FUNCTION mgd.all_allele_delete();


--
-- Name: all_allele all_allele_insert_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER all_allele_insert_trigger AFTER INSERT ON mgd.all_allele FOR EACH ROW EXECUTE FUNCTION mgd.all_allele_insert();


--
-- Name: all_allele all_allele_update_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER all_allele_update_trigger AFTER UPDATE ON mgd.all_allele FOR EACH ROW EXECUTE FUNCTION mgd.all_allele_update();


--
-- Name: all_cellline all_cellline_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER all_cellline_delete_trigger AFTER DELETE ON mgd.all_cellline FOR EACH ROW EXECUTE FUNCTION mgd.all_cellline_delete();


--
-- Name: all_cellline all_cellline_update1_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER all_cellline_update1_trigger AFTER UPDATE OF _strain_key ON mgd.all_cellline FOR EACH ROW EXECUTE FUNCTION mgd.all_cellline_update1();


--
-- Name: all_cellline all_cellline_update2_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER all_cellline_update2_trigger AFTER UPDATE OF _derivation_key ON mgd.all_cellline FOR EACH ROW EXECUTE FUNCTION mgd.all_cellline_update2();


--
-- Name: all_variant all_variant_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER all_variant_delete_trigger AFTER DELETE ON mgd.all_variant FOR EACH ROW EXECUTE FUNCTION mgd.all_variant_delete();


--
-- Name: bib_refs bib_refs_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER bib_refs_delete_trigger AFTER DELETE ON mgd.bib_refs FOR EACH ROW EXECUTE FUNCTION mgd.bib_refs_delete();


--
-- Name: bib_refs bib_refs_insert_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER bib_refs_insert_trigger AFTER INSERT ON mgd.bib_refs FOR EACH ROW EXECUTE FUNCTION mgd.bib_refs_insert();


--
-- Name: gxd_allelepair gxd_allelepair_insert_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER gxd_allelepair_insert_trigger AFTER INSERT OR UPDATE ON mgd.gxd_allelepair FOR EACH ROW EXECUTE FUNCTION mgd.gxd_allelepair_insert();


--
-- Name: gxd_antibody gxd_antibody_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER gxd_antibody_delete_trigger BEFORE DELETE ON mgd.gxd_antibody FOR EACH ROW EXECUTE FUNCTION mgd.gxd_antibody_delete();


--
-- Name: gxd_antibody gxd_antibody_insert_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER gxd_antibody_insert_trigger AFTER INSERT ON mgd.gxd_antibody FOR EACH ROW EXECUTE FUNCTION mgd.gxd_antibody_insert();


--
-- Name: gxd_antigen gxd_antigen_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER gxd_antigen_delete_trigger AFTER DELETE ON mgd.gxd_antigen FOR EACH ROW EXECUTE FUNCTION mgd.gxd_antigen_delete();


--
-- Name: gxd_antigen gxd_antigen_insert_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER gxd_antigen_insert_trigger AFTER INSERT ON mgd.gxd_antigen FOR EACH ROW EXECUTE FUNCTION mgd.gxd_antigen_insert();


--
-- Name: gxd_assay gxd_assay_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER gxd_assay_delete_trigger AFTER DELETE ON mgd.gxd_assay FOR EACH ROW EXECUTE FUNCTION mgd.gxd_assay_delete();


--
-- Name: gxd_assay gxd_assay_insert_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER gxd_assay_insert_trigger AFTER INSERT ON mgd.gxd_assay FOR EACH ROW EXECUTE FUNCTION mgd.gxd_assay_insert();


--
-- Name: gxd_gelrow gxd_gelrow_insert_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER gxd_gelrow_insert_trigger AFTER INSERT OR UPDATE ON mgd.gxd_gelrow FOR EACH ROW EXECUTE FUNCTION mgd.gxd_gelrow_insert();


--
-- Name: gxd_genotype gxd_genotype_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER gxd_genotype_delete_trigger AFTER DELETE ON mgd.gxd_genotype FOR EACH ROW EXECUTE FUNCTION mgd.gxd_genotype_delete();


--
-- Name: gxd_genotype gxd_genotype_insert_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER gxd_genotype_insert_trigger AFTER INSERT ON mgd.gxd_genotype FOR EACH ROW EXECUTE FUNCTION mgd.gxd_genotype_insert();


--
-- Name: gxd_htexperiment gxd_htexperiment_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER gxd_htexperiment_delete_trigger AFTER DELETE ON mgd.gxd_htexperiment FOR EACH ROW EXECUTE FUNCTION mgd.gxd_htexperiment_delete();


--
-- Name: gxd_htrawsample gxd_htrawsample_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER gxd_htrawsample_delete_trigger AFTER DELETE ON mgd.gxd_htrawsample FOR EACH ROW EXECUTE FUNCTION mgd.gxd_htrawsample_delete();


--
-- Name: gxd_htsample gxd_htsample_ageminmax_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER gxd_htsample_ageminmax_trigger AFTER INSERT OR UPDATE OF age ON mgd.gxd_htsample FOR EACH ROW EXECUTE FUNCTION mgd.gxd_htsample_ageminmax();


--
-- Name: gxd_index gxd_index_insert_before_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER gxd_index_insert_before_trigger BEFORE INSERT ON mgd.gxd_index FOR EACH ROW EXECUTE FUNCTION mgd.gxd_index_insert_before();


--
-- Name: gxd_index gxd_index_insert_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER gxd_index_insert_trigger AFTER INSERT ON mgd.gxd_index FOR EACH ROW EXECUTE FUNCTION mgd.gxd_index_insert();


--
-- Name: gxd_index gxd_index_update_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER gxd_index_update_trigger AFTER UPDATE OF _priority_key, _conditionalmutants_key ON mgd.gxd_index FOR EACH ROW WHEN ((pg_trigger_depth() < 1)) EXECUTE FUNCTION mgd.gxd_index_update();


--
-- Name: img_image img_image_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER img_image_delete_trigger AFTER DELETE ON mgd.img_image FOR EACH ROW EXECUTE FUNCTION mgd.img_image_delete();


--
-- Name: img_image img_image_insert_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER img_image_insert_trigger AFTER INSERT ON mgd.img_image FOR EACH ROW EXECUTE FUNCTION mgd.img_image_insert();


--
-- Name: mgi_organism mgi_organism_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER mgi_organism_delete_trigger AFTER DELETE ON mgd.mgi_organism FOR EACH ROW EXECUTE FUNCTION mgd.mgi_organism_delete();


--
-- Name: mgi_organism mgi_organism_insert_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER mgi_organism_insert_trigger AFTER INSERT ON mgd.mgi_organism FOR EACH ROW EXECUTE FUNCTION mgd.mgi_organism_insert();


--
-- Name: mgi_reference_assoc mgi_reference_assoc_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER mgi_reference_assoc_delete_trigger AFTER DELETE ON mgd.mgi_reference_assoc FOR EACH ROW EXECUTE FUNCTION mgd.mgi_reference_assoc_delete();


--
-- Name: mgi_reference_assoc mgi_reference_assoc_insert_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER mgi_reference_assoc_insert_trigger AFTER INSERT ON mgd.mgi_reference_assoc FOR EACH ROW EXECUTE FUNCTION mgd.mgi_reference_assoc_insert();


--
-- Name: mgi_relationship mgi_relationship_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER mgi_relationship_delete_trigger AFTER DELETE ON mgd.mgi_relationship FOR EACH ROW EXECUTE FUNCTION mgd.mgi_relationship_delete();


--
-- Name: mgi_setmember_emapa mgi_setmember_emapa_insert_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER mgi_setmember_emapa_insert_trigger AFTER INSERT ON mgd.mgi_setmember_emapa FOR EACH ROW EXECUTE FUNCTION mgd.mgi_setmember_emapa_insert();


--
-- Name: mgi_setmember_emapa mgi_setmember_emapa_update_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER mgi_setmember_emapa_update_trigger AFTER UPDATE ON mgd.mgi_setmember_emapa FOR EACH ROW EXECUTE FUNCTION mgd.mgi_setmember_emapa_update();


--
-- Name: mld_expt_marker mld_expt_marker_update_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER mld_expt_marker_update_trigger AFTER UPDATE OF _marker_key ON mgd.mld_expt_marker FOR EACH ROW EXECUTE FUNCTION mgd.mld_expt_marker_update();


--
-- Name: mld_expts mld_expts_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER mld_expts_delete_trigger AFTER DELETE ON mgd.mld_expts FOR EACH ROW EXECUTE FUNCTION mgd.mld_expts_delete();


--
-- Name: mld_expts mld_expts_insert_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER mld_expts_insert_trigger AFTER INSERT ON mgd.mld_expts FOR EACH ROW EXECUTE FUNCTION mgd.mld_expts_insert();


--
-- Name: mrk_cluster mrk_cluster_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER mrk_cluster_delete_trigger AFTER DELETE ON mgd.mrk_cluster FOR EACH ROW EXECUTE FUNCTION mgd.mrk_cluster_delete();


--
-- Name: mrk_marker mrk_marker_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER mrk_marker_delete_trigger AFTER DELETE ON mgd.mrk_marker FOR EACH ROW EXECUTE FUNCTION mgd.mrk_marker_delete();


--
-- Name: mrk_marker mrk_marker_insert_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER mrk_marker_insert_trigger AFTER INSERT ON mgd.mrk_marker FOR EACH ROW EXECUTE FUNCTION mgd.mrk_marker_insert();


--
-- Name: mrk_marker mrk_marker_update_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER mrk_marker_update_trigger AFTER UPDATE ON mgd.mrk_marker FOR EACH ROW EXECUTE FUNCTION mgd.mrk_marker_update();


--
-- Name: mrk_strainmarker mrk_strainmarker_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER mrk_strainmarker_delete_trigger AFTER DELETE ON mgd.mrk_strainmarker FOR EACH ROW EXECUTE FUNCTION mgd.mrk_strainmarker_delete();


--
-- Name: prb_marker prb_marker_insert_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER prb_marker_insert_trigger AFTER INSERT ON mgd.prb_marker FOR EACH ROW EXECUTE FUNCTION mgd.prb_marker_insert();


--
-- Name: prb_marker prb_marker_update_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER prb_marker_update_trigger BEFORE UPDATE ON mgd.prb_marker FOR EACH ROW EXECUTE FUNCTION mgd.prb_marker_update();


--
-- Name: prb_probe prb_probe_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER prb_probe_delete_trigger BEFORE DELETE ON mgd.prb_probe FOR EACH ROW EXECUTE FUNCTION mgd.prb_probe_delete();


--
-- Name: prb_probe prb_probe_insert_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER prb_probe_insert_trigger AFTER INSERT ON mgd.prb_probe FOR EACH ROW EXECUTE FUNCTION mgd.prb_probe_insert();


--
-- Name: prb_reference prb_reference_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER prb_reference_delete_trigger AFTER DELETE ON mgd.prb_reference FOR EACH ROW EXECUTE FUNCTION mgd.prb_reference_delete();


--
-- Name: prb_reference prb_reference_update_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER prb_reference_update_trigger AFTER UPDATE ON mgd.prb_reference FOR EACH ROW EXECUTE FUNCTION mgd.prb_reference_update();


--
-- Name: prb_source prb_source_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER prb_source_delete_trigger AFTER DELETE ON mgd.prb_source FOR EACH ROW EXECUTE FUNCTION mgd.prb_source_delete();


--
-- Name: prb_strain prb_strain_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER prb_strain_delete_trigger AFTER DELETE ON mgd.prb_strain FOR EACH ROW EXECUTE FUNCTION mgd.prb_strain_delete();


--
-- Name: prb_strain prb_strain_insert_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER prb_strain_insert_trigger AFTER INSERT ON mgd.prb_strain FOR EACH ROW EXECUTE FUNCTION mgd.prb_strain_insert();


--
-- Name: seq_sequence seq_sequence_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER seq_sequence_delete_trigger AFTER DELETE ON mgd.seq_sequence FOR EACH ROW EXECUTE FUNCTION mgd.seq_sequence_delete();


--
-- Name: voc_annot voc_annot_insert_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER voc_annot_insert_trigger AFTER INSERT ON mgd.voc_annot FOR EACH ROW EXECUTE FUNCTION mgd.voc_annot_insert();


--
-- Name: voc_evidence voc_evidence_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER voc_evidence_delete_trigger AFTER DELETE ON mgd.voc_evidence FOR EACH ROW EXECUTE FUNCTION mgd.voc_evidence_delete();


--
-- Name: voc_evidence voc_evidence_insert_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER voc_evidence_insert_trigger AFTER INSERT OR UPDATE ON mgd.voc_evidence FOR EACH ROW EXECUTE FUNCTION mgd.voc_evidence_insert();


--
-- Name: voc_evidence_property voc_evidence_property_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER voc_evidence_property_delete_trigger AFTER DELETE ON mgd.voc_evidence_property FOR EACH ROW EXECUTE FUNCTION mgd.voc_evidence_property_delete();


--
-- Name: voc_evidence voc_evidence_update_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER voc_evidence_update_trigger AFTER UPDATE ON mgd.voc_evidence FOR EACH ROW EXECUTE FUNCTION mgd.voc_evidence_update();


--
-- Name: voc_term voc_term_delete_trigger; Type: TRIGGER; Schema: mgd; Owner: -
--

CREATE TRIGGER voc_term_delete_trigger AFTER DELETE ON mgd.voc_term FOR EACH ROW EXECUTE FUNCTION mgd.voc_term_delete();


--
-- Name: acc_accession acc_accession__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.acc_accession
    ADD CONSTRAINT acc_accession__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: acc_accession acc_accession__logicaldb_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.acc_accession
    ADD CONSTRAINT acc_accession__logicaldb_key_fkey FOREIGN KEY (_logicaldb_key) REFERENCES mgd.acc_logicaldb(_logicaldb_key) DEFERRABLE;


--
-- Name: acc_accession acc_accession__mgitype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.acc_accession
    ADD CONSTRAINT acc_accession__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


--
-- Name: acc_accession acc_accession__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.acc_accession
    ADD CONSTRAINT acc_accession__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: acc_accessionreference acc_accessionreference__accession_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.acc_accessionreference
    ADD CONSTRAINT acc_accessionreference__accession_key_fkey FOREIGN KEY (_accession_key) REFERENCES mgd.acc_accession(_accession_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: acc_accessionreference acc_accessionreference__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.acc_accessionreference
    ADD CONSTRAINT acc_accessionreference__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: acc_accessionreference acc_accessionreference__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.acc_accessionreference
    ADD CONSTRAINT acc_accessionreference__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: acc_accessionreference acc_accessionreference__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.acc_accessionreference
    ADD CONSTRAINT acc_accessionreference__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: acc_actualdb acc_actualdb__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.acc_actualdb
    ADD CONSTRAINT acc_actualdb__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: acc_actualdb acc_actualdb__logicaldb_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.acc_actualdb
    ADD CONSTRAINT acc_actualdb__logicaldb_key_fkey FOREIGN KEY (_logicaldb_key) REFERENCES mgd.acc_logicaldb(_logicaldb_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: acc_actualdb acc_actualdb__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.acc_actualdb
    ADD CONSTRAINT acc_actualdb__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: acc_logicaldb acc_logicaldb__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.acc_logicaldb
    ADD CONSTRAINT acc_logicaldb__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: acc_logicaldb acc_logicaldb__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.acc_logicaldb
    ADD CONSTRAINT acc_logicaldb__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: acc_logicaldb acc_logicaldb__organism_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.acc_logicaldb
    ADD CONSTRAINT acc_logicaldb__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


--
-- Name: acc_mgitype acc_mgitype__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.acc_mgitype
    ADD CONSTRAINT acc_mgitype__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: acc_mgitype acc_mgitype__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.acc_mgitype
    ADD CONSTRAINT acc_mgitype__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: all_allele all_allele__allele_status_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele__allele_status_key_fkey FOREIGN KEY (_allele_status_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: all_allele all_allele__allele_type_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele__allele_type_key_fkey FOREIGN KEY (_allele_type_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: all_allele all_allele__approvedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele__approvedby_key_fkey FOREIGN KEY (_approvedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: all_allele all_allele__collection_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele__collection_key_fkey FOREIGN KEY (_collection_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: all_allele all_allele__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: all_allele all_allele__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- Name: all_allele all_allele__markerallele_status_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele__markerallele_status_key_fkey FOREIGN KEY (_markerallele_status_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: all_allele all_allele__mode_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele__mode_key_fkey FOREIGN KEY (_mode_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: all_allele all_allele__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: all_allele all_allele__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


--
-- Name: all_allele all_allele__strain_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele__strain_key_fkey FOREIGN KEY (_strain_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


--
-- Name: all_allele all_allele__transmission_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_allele
    ADD CONSTRAINT all_allele__transmission_key_fkey FOREIGN KEY (_transmission_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: all_allele_cellline all_allele_cellline__allele_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_allele_cellline
    ADD CONSTRAINT all_allele_cellline__allele_key_fkey FOREIGN KEY (_allele_key) REFERENCES mgd.all_allele(_allele_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: all_allele_cellline all_allele_cellline__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_allele_cellline
    ADD CONSTRAINT all_allele_cellline__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: all_allele_cellline all_allele_cellline__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_allele_cellline
    ADD CONSTRAINT all_allele_cellline__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: all_allele_cellline all_allele_cellline__mutantcellline_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_allele_cellline
    ADD CONSTRAINT all_allele_cellline__mutantcellline_key_fkey FOREIGN KEY (_mutantcellline_key) REFERENCES mgd.all_cellline(_cellline_key) DEFERRABLE;


--
-- Name: all_allele_mutation all_allele_mutation__allele_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_allele_mutation
    ADD CONSTRAINT all_allele_mutation__allele_key_fkey FOREIGN KEY (_allele_key) REFERENCES mgd.all_allele(_allele_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: all_allele_mutation all_allele_mutation__mutation_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_allele_mutation
    ADD CONSTRAINT all_allele_mutation__mutation_key_fkey FOREIGN KEY (_mutation_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: all_cellline all_cellline__cellline_type_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_cellline
    ADD CONSTRAINT all_cellline__cellline_type_key_fkey FOREIGN KEY (_cellline_type_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: all_cellline all_cellline__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_cellline
    ADD CONSTRAINT all_cellline__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: all_cellline all_cellline__derivation_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_cellline
    ADD CONSTRAINT all_cellline__derivation_key_fkey FOREIGN KEY (_derivation_key) REFERENCES mgd.all_cellline_derivation(_derivation_key) DEFERRABLE;


--
-- Name: all_cellline all_cellline__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_cellline
    ADD CONSTRAINT all_cellline__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: all_cellline all_cellline__strain_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_cellline
    ADD CONSTRAINT all_cellline__strain_key_fkey FOREIGN KEY (_strain_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


--
-- Name: all_cellline_derivation all_cellline_derivation__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_cellline_derivation
    ADD CONSTRAINT all_cellline_derivation__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: all_cellline_derivation all_cellline_derivation__creator_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_cellline_derivation
    ADD CONSTRAINT all_cellline_derivation__creator_key_fkey FOREIGN KEY (_creator_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: all_cellline_derivation all_cellline_derivation__derivationtype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_cellline_derivation
    ADD CONSTRAINT all_cellline_derivation__derivationtype_key_fkey FOREIGN KEY (_derivationtype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: all_cellline_derivation all_cellline_derivation__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_cellline_derivation
    ADD CONSTRAINT all_cellline_derivation__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: all_cellline_derivation all_cellline_derivation__parentcellline_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_cellline_derivation
    ADD CONSTRAINT all_cellline_derivation__parentcellline_key_fkey FOREIGN KEY (_parentcellline_key) REFERENCES mgd.all_cellline(_cellline_key) DEFERRABLE;


--
-- Name: all_cellline_derivation all_cellline_derivation__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_cellline_derivation
    ADD CONSTRAINT all_cellline_derivation__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


--
-- Name: all_cellline_derivation all_cellline_derivation__vector_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_cellline_derivation
    ADD CONSTRAINT all_cellline_derivation__vector_key_fkey FOREIGN KEY (_vector_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: all_cellline_derivation all_cellline_derivation__vectortype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_cellline_derivation
    ADD CONSTRAINT all_cellline_derivation__vectortype_key_fkey FOREIGN KEY (_vectortype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: all_cre_cache all_cre_cache__allele_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_cre_cache
    ADD CONSTRAINT all_cre_cache__allele_key_fkey FOREIGN KEY (_allele_key) REFERENCES mgd.all_allele(_allele_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: all_cre_cache all_cre_cache__allele_type_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_cre_cache
    ADD CONSTRAINT all_cre_cache__allele_type_key_fkey FOREIGN KEY (_allele_type_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: all_cre_cache all_cre_cache__assay_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_cre_cache
    ADD CONSTRAINT all_cre_cache__assay_key_fkey FOREIGN KEY (_assay_key) REFERENCES mgd.gxd_assay(_assay_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: all_cre_cache all_cre_cache__celltype_term_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_cre_cache
    ADD CONSTRAINT all_cre_cache__celltype_term_key_fkey FOREIGN KEY (_celltype_term_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: all_cre_cache all_cre_cache__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_cre_cache
    ADD CONSTRAINT all_cre_cache__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: all_cre_cache all_cre_cache__emapa_term_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_cre_cache
    ADD CONSTRAINT all_cre_cache__emapa_term_key_fkey FOREIGN KEY (_emapa_term_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: all_cre_cache all_cre_cache__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_cre_cache
    ADD CONSTRAINT all_cre_cache__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: all_cre_cache all_cre_cache__stage_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_cre_cache
    ADD CONSTRAINT all_cre_cache__stage_key_fkey FOREIGN KEY (_stage_key) REFERENCES mgd.gxd_theilerstage(_stage_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: all_knockout_cache all_knockout_cache__allele_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_knockout_cache
    ADD CONSTRAINT all_knockout_cache__allele_key_fkey FOREIGN KEY (_allele_key) REFERENCES mgd.all_allele(_allele_key) DEFERRABLE;


--
-- Name: all_knockout_cache all_knockout_cache__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_knockout_cache
    ADD CONSTRAINT all_knockout_cache__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: all_knockout_cache all_knockout_cache__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_knockout_cache
    ADD CONSTRAINT all_knockout_cache__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: all_knockout_cache all_knockout_cache__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_knockout_cache
    ADD CONSTRAINT all_knockout_cache__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: all_label all_label__allele_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_label
    ADD CONSTRAINT all_label__allele_key_fkey FOREIGN KEY (_allele_key) REFERENCES mgd.all_allele(_allele_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: all_variant all_variant__allele_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_variant
    ADD CONSTRAINT all_variant__allele_key_fkey FOREIGN KEY (_allele_key) REFERENCES mgd.all_allele(_allele_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: all_variant all_variant__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_variant
    ADD CONSTRAINT all_variant__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: all_variant all_variant__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_variant
    ADD CONSTRAINT all_variant__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: all_variant all_variant__sourcevariant_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_variant
    ADD CONSTRAINT all_variant__sourcevariant_key_fkey FOREIGN KEY (_sourcevariant_key) REFERENCES mgd.all_variant(_variant_key) DEFERRABLE;


--
-- Name: all_variant all_variant__strain_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_variant
    ADD CONSTRAINT all_variant__strain_key_fkey FOREIGN KEY (_strain_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


--
-- Name: all_variant_sequence all_variant_sequence__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_variant_sequence
    ADD CONSTRAINT all_variant_sequence__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: all_variant_sequence all_variant_sequence__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_variant_sequence
    ADD CONSTRAINT all_variant_sequence__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: all_variant_sequence all_variant_sequence__sequence_type_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_variant_sequence
    ADD CONSTRAINT all_variant_sequence__sequence_type_key_fkey FOREIGN KEY (_sequence_type_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: all_variant_sequence all_variant_sequence__variant_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.all_variant_sequence
    ADD CONSTRAINT all_variant_sequence__variant_key_fkey FOREIGN KEY (_variant_key) REFERENCES mgd.all_variant(_variant_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: bib_books bib_books__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_books
    ADD CONSTRAINT bib_books__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: bib_citation_cache bib_citation_cache__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_citation_cache
    ADD CONSTRAINT bib_citation_cache__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: bib_citation_cache bib_citation_cache__relevance_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_citation_cache
    ADD CONSTRAINT bib_citation_cache__relevance_key_fkey FOREIGN KEY (_relevance_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: bib_notes bib_notes__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_notes
    ADD CONSTRAINT bib_notes__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: bib_refs bib_refs__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_refs
    ADD CONSTRAINT bib_refs__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: bib_refs bib_refs__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_refs
    ADD CONSTRAINT bib_refs__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: bib_refs bib_refs__referencetype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_refs
    ADD CONSTRAINT bib_refs__referencetype_key_fkey FOREIGN KEY (_referencetype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: bib_workflow_data bib_workflow_data__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_workflow_data
    ADD CONSTRAINT bib_workflow_data__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: bib_workflow_data bib_workflow_data__extractedtext_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_workflow_data
    ADD CONSTRAINT bib_workflow_data__extractedtext_key_fkey FOREIGN KEY (_extractedtext_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: bib_workflow_data bib_workflow_data__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_workflow_data
    ADD CONSTRAINT bib_workflow_data__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: bib_workflow_data bib_workflow_data__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_workflow_data
    ADD CONSTRAINT bib_workflow_data__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: bib_workflow_data bib_workflow_data__supplemental_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_workflow_data
    ADD CONSTRAINT bib_workflow_data__supplemental_key_fkey FOREIGN KEY (_supplemental_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: bib_workflow_relevance bib_workflow_relevance__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_workflow_relevance
    ADD CONSTRAINT bib_workflow_relevance__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: bib_workflow_relevance bib_workflow_relevance__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_workflow_relevance
    ADD CONSTRAINT bib_workflow_relevance__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: bib_workflow_relevance bib_workflow_relevance__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_workflow_relevance
    ADD CONSTRAINT bib_workflow_relevance__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: bib_workflow_relevance bib_workflow_relevance__relevance_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_workflow_relevance
    ADD CONSTRAINT bib_workflow_relevance__relevance_key_fkey FOREIGN KEY (_relevance_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: bib_workflow_status bib_workflow_status__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_workflow_status
    ADD CONSTRAINT bib_workflow_status__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: bib_workflow_status bib_workflow_status__group_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_workflow_status
    ADD CONSTRAINT bib_workflow_status__group_key_fkey FOREIGN KEY (_group_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: bib_workflow_status bib_workflow_status__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_workflow_status
    ADD CONSTRAINT bib_workflow_status__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: bib_workflow_status bib_workflow_status__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_workflow_status
    ADD CONSTRAINT bib_workflow_status__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: bib_workflow_status bib_workflow_status__status_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_workflow_status
    ADD CONSTRAINT bib_workflow_status__status_key_fkey FOREIGN KEY (_status_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: bib_workflow_tag bib_workflow_tag__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_workflow_tag
    ADD CONSTRAINT bib_workflow_tag__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: bib_workflow_tag bib_workflow_tag__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_workflow_tag
    ADD CONSTRAINT bib_workflow_tag__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: bib_workflow_tag bib_workflow_tag__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_workflow_tag
    ADD CONSTRAINT bib_workflow_tag__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: bib_workflow_tag bib_workflow_tag__tag_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.bib_workflow_tag
    ADD CONSTRAINT bib_workflow_tag__tag_key_fkey FOREIGN KEY (_tag_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: crs_cross crs_cross__femalestrain_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.crs_cross
    ADD CONSTRAINT crs_cross__femalestrain_key_fkey FOREIGN KEY (_femalestrain_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


--
-- Name: crs_cross crs_cross__malestrain_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.crs_cross
    ADD CONSTRAINT crs_cross__malestrain_key_fkey FOREIGN KEY (_malestrain_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


--
-- Name: crs_cross crs_cross__strainho_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.crs_cross
    ADD CONSTRAINT crs_cross__strainho_key_fkey FOREIGN KEY (_strainho_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


--
-- Name: crs_cross crs_cross__strainht_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.crs_cross
    ADD CONSTRAINT crs_cross__strainht_key_fkey FOREIGN KEY (_strainht_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


--
-- Name: crs_matrix crs_matrix__cross_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.crs_matrix
    ADD CONSTRAINT crs_matrix__cross_key_fkey FOREIGN KEY (_cross_key) REFERENCES mgd.crs_cross(_cross_key) DEFERRABLE;


--
-- Name: crs_matrix crs_matrix__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.crs_matrix
    ADD CONSTRAINT crs_matrix__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- Name: crs_progeny crs_progeny__cross_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.crs_progeny
    ADD CONSTRAINT crs_progeny__cross_key_fkey FOREIGN KEY (_cross_key) REFERENCES mgd.crs_cross(_cross_key) DEFERRABLE;


--
-- Name: crs_references crs_references__cross_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.crs_references
    ADD CONSTRAINT crs_references__cross_key_fkey FOREIGN KEY (_cross_key) REFERENCES mgd.crs_cross(_cross_key) DEFERRABLE;


--
-- Name: crs_references crs_references__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.crs_references
    ADD CONSTRAINT crs_references__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- Name: crs_references crs_references__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.crs_references
    ADD CONSTRAINT crs_references__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


--
-- Name: crs_typings crs_typings__cross_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.crs_typings
    ADD CONSTRAINT crs_typings__cross_key_fkey FOREIGN KEY (_cross_key, rownumber) REFERENCES mgd.crs_matrix(_cross_key, rownumber) DEFERRABLE;


--
-- Name: dag_closure dag_closure__ancestor_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.dag_closure
    ADD CONSTRAINT dag_closure__ancestor_key_fkey FOREIGN KEY (_ancestor_key) REFERENCES mgd.dag_node(_node_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: dag_closure dag_closure__ancestorlabel_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.dag_closure
    ADD CONSTRAINT dag_closure__ancestorlabel_key_fkey FOREIGN KEY (_ancestorlabel_key) REFERENCES mgd.dag_label(_label_key) DEFERRABLE;


--
-- Name: dag_closure dag_closure__dag_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.dag_closure
    ADD CONSTRAINT dag_closure__dag_key_fkey FOREIGN KEY (_dag_key) REFERENCES mgd.dag_dag(_dag_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: dag_closure dag_closure__descendent_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.dag_closure
    ADD CONSTRAINT dag_closure__descendent_key_fkey FOREIGN KEY (_descendent_key) REFERENCES mgd.dag_node(_node_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: dag_closure dag_closure__descendentlabel_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.dag_closure
    ADD CONSTRAINT dag_closure__descendentlabel_key_fkey FOREIGN KEY (_descendentlabel_key) REFERENCES mgd.dag_label(_label_key) DEFERRABLE;


--
-- Name: dag_closure dag_closure__mgitype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.dag_closure
    ADD CONSTRAINT dag_closure__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


--
-- Name: dag_dag dag_dag__mgitype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.dag_dag
    ADD CONSTRAINT dag_dag__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


--
-- Name: dag_dag dag_dag__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.dag_dag
    ADD CONSTRAINT dag_dag__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


--
-- Name: dag_edge dag_edge__child_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.dag_edge
    ADD CONSTRAINT dag_edge__child_key_fkey FOREIGN KEY (_child_key) REFERENCES mgd.dag_node(_node_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: dag_edge dag_edge__dag_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.dag_edge
    ADD CONSTRAINT dag_edge__dag_key_fkey FOREIGN KEY (_dag_key) REFERENCES mgd.dag_dag(_dag_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: dag_edge dag_edge__label_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.dag_edge
    ADD CONSTRAINT dag_edge__label_key_fkey FOREIGN KEY (_label_key) REFERENCES mgd.dag_label(_label_key) DEFERRABLE;


--
-- Name: dag_edge dag_edge__parent_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.dag_edge
    ADD CONSTRAINT dag_edge__parent_key_fkey FOREIGN KEY (_parent_key) REFERENCES mgd.dag_node(_node_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: dag_node dag_node__dag_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.dag_node
    ADD CONSTRAINT dag_node__dag_key_fkey FOREIGN KEY (_dag_key) REFERENCES mgd.dag_dag(_dag_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: dag_node dag_node__label_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.dag_node
    ADD CONSTRAINT dag_node__label_key_fkey FOREIGN KEY (_label_key) REFERENCES mgd.dag_label(_label_key) DEFERRABLE;


--
-- Name: go_tracking go_tracking__completedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.go_tracking
    ADD CONSTRAINT go_tracking__completedby_key_fkey FOREIGN KEY (_completedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: go_tracking go_tracking__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.go_tracking
    ADD CONSTRAINT go_tracking__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: go_tracking go_tracking__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.go_tracking
    ADD CONSTRAINT go_tracking__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: go_tracking go_tracking__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.go_tracking
    ADD CONSTRAINT go_tracking__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_allelegenotype gxd_allelegenotype__allele_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_allelegenotype
    ADD CONSTRAINT gxd_allelegenotype__allele_key_fkey FOREIGN KEY (_allele_key) REFERENCES mgd.all_allele(_allele_key) DEFERRABLE;


--
-- Name: gxd_allelegenotype gxd_allelegenotype__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_allelegenotype
    ADD CONSTRAINT gxd_allelegenotype__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_allelegenotype gxd_allelegenotype__genotype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_allelegenotype
    ADD CONSTRAINT gxd_allelegenotype__genotype_key_fkey FOREIGN KEY (_genotype_key) REFERENCES mgd.gxd_genotype(_genotype_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_allelegenotype gxd_allelegenotype__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_allelegenotype
    ADD CONSTRAINT gxd_allelegenotype__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- Name: gxd_allelegenotype gxd_allelegenotype__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_allelegenotype
    ADD CONSTRAINT gxd_allelegenotype__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_allelepair gxd_allelepair__allele_key_1_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_allelepair
    ADD CONSTRAINT gxd_allelepair__allele_key_1_fkey FOREIGN KEY (_allele_key_1) REFERENCES mgd.all_allele(_allele_key) DEFERRABLE;


--
-- Name: gxd_allelepair gxd_allelepair__allele_key_2_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_allelepair
    ADD CONSTRAINT gxd_allelepair__allele_key_2_fkey FOREIGN KEY (_allele_key_2) REFERENCES mgd.all_allele(_allele_key) DEFERRABLE;


--
-- Name: gxd_allelepair gxd_allelepair__compound_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_allelepair
    ADD CONSTRAINT gxd_allelepair__compound_key_fkey FOREIGN KEY (_compound_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_allelepair gxd_allelepair__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_allelepair
    ADD CONSTRAINT gxd_allelepair__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_allelepair gxd_allelepair__genotype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_allelepair
    ADD CONSTRAINT gxd_allelepair__genotype_key_fkey FOREIGN KEY (_genotype_key) REFERENCES mgd.gxd_genotype(_genotype_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_allelepair gxd_allelepair__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_allelepair
    ADD CONSTRAINT gxd_allelepair__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- Name: gxd_allelepair gxd_allelepair__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_allelepair
    ADD CONSTRAINT gxd_allelepair__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_allelepair gxd_allelepair__mutantcellline_key_1_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_allelepair
    ADD CONSTRAINT gxd_allelepair__mutantcellline_key_1_fkey FOREIGN KEY (_mutantcellline_key_1) REFERENCES mgd.all_cellline(_cellline_key) DEFERRABLE;


--
-- Name: gxd_allelepair gxd_allelepair__mutantcellline_key_2_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_allelepair
    ADD CONSTRAINT gxd_allelepair__mutantcellline_key_2_fkey FOREIGN KEY (_mutantcellline_key_2) REFERENCES mgd.all_cellline(_cellline_key) DEFERRABLE;


--
-- Name: gxd_allelepair gxd_allelepair__pairstate_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_allelepair
    ADD CONSTRAINT gxd_allelepair__pairstate_key_fkey FOREIGN KEY (_pairstate_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_antibody gxd_antibody__antibodyclass_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_antibody
    ADD CONSTRAINT gxd_antibody__antibodyclass_key_fkey FOREIGN KEY (_antibodyclass_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_antibody gxd_antibody__antibodytype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_antibody
    ADD CONSTRAINT gxd_antibody__antibodytype_key_fkey FOREIGN KEY (_antibodytype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_antibody gxd_antibody__antigen_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_antibody
    ADD CONSTRAINT gxd_antibody__antigen_key_fkey FOREIGN KEY (_antigen_key) REFERENCES mgd.gxd_antigen(_antigen_key) DEFERRABLE;


--
-- Name: gxd_antibody gxd_antibody__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_antibody
    ADD CONSTRAINT gxd_antibody__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_antibody gxd_antibody__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_antibody
    ADD CONSTRAINT gxd_antibody__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_antibody gxd_antibody__organism_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_antibody
    ADD CONSTRAINT gxd_antibody__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


--
-- Name: gxd_antibodyalias gxd_antibodyalias__antibody_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_antibodyalias
    ADD CONSTRAINT gxd_antibodyalias__antibody_key_fkey FOREIGN KEY (_antibody_key) REFERENCES mgd.gxd_antibody(_antibody_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_antibodyalias gxd_antibodyalias__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_antibodyalias
    ADD CONSTRAINT gxd_antibodyalias__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


--
-- Name: gxd_antibodymarker gxd_antibodymarker__antibody_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_antibodymarker
    ADD CONSTRAINT gxd_antibodymarker__antibody_key_fkey FOREIGN KEY (_antibody_key) REFERENCES mgd.gxd_antibody(_antibody_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_antibodymarker gxd_antibodymarker__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_antibodymarker
    ADD CONSTRAINT gxd_antibodymarker__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- Name: gxd_antibodyprep gxd_antibodyprep__antibody_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_antibodyprep
    ADD CONSTRAINT gxd_antibodyprep__antibody_key_fkey FOREIGN KEY (_antibody_key) REFERENCES mgd.gxd_antibody(_antibody_key) DEFERRABLE;


--
-- Name: gxd_antibodyprep gxd_antibodyprep__label_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_antibodyprep
    ADD CONSTRAINT gxd_antibodyprep__label_key_fkey FOREIGN KEY (_label_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_antibodyprep gxd_antibodyprep__secondary_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_antibodyprep
    ADD CONSTRAINT gxd_antibodyprep__secondary_key_fkey FOREIGN KEY (_secondary_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_antigen gxd_antigen__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_antigen
    ADD CONSTRAINT gxd_antigen__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_antigen gxd_antigen__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_antigen
    ADD CONSTRAINT gxd_antigen__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_antigen gxd_antigen__source_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_antigen
    ADD CONSTRAINT gxd_antigen__source_key_fkey FOREIGN KEY (_source_key) REFERENCES mgd.prb_source(_source_key) DEFERRABLE;


--
-- Name: gxd_assay gxd_assay__antibodyprep_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_assay
    ADD CONSTRAINT gxd_assay__antibodyprep_key_fkey FOREIGN KEY (_antibodyprep_key) REFERENCES mgd.gxd_antibodyprep(_antibodyprep_key) DEFERRABLE;


--
-- Name: gxd_assay gxd_assay__assaytype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_assay
    ADD CONSTRAINT gxd_assay__assaytype_key_fkey FOREIGN KEY (_assaytype_key) REFERENCES mgd.gxd_assaytype(_assaytype_key) DEFERRABLE;


--
-- Name: gxd_assay gxd_assay__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_assay
    ADD CONSTRAINT gxd_assay__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_assay gxd_assay__imagepane_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_assay
    ADD CONSTRAINT gxd_assay__imagepane_key_fkey FOREIGN KEY (_imagepane_key) REFERENCES mgd.img_imagepane(_imagepane_key) DEFERRABLE;


--
-- Name: gxd_assay gxd_assay__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_assay
    ADD CONSTRAINT gxd_assay__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- Name: gxd_assay gxd_assay__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_assay
    ADD CONSTRAINT gxd_assay__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_assay gxd_assay__probeprep_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_assay
    ADD CONSTRAINT gxd_assay__probeprep_key_fkey FOREIGN KEY (_probeprep_key) REFERENCES mgd.gxd_probeprep(_probeprep_key) DEFERRABLE;


--
-- Name: gxd_assay gxd_assay__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_assay
    ADD CONSTRAINT gxd_assay__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


--
-- Name: gxd_assay gxd_assay__reportergene_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_assay
    ADD CONSTRAINT gxd_assay__reportergene_key_fkey FOREIGN KEY (_reportergene_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_assaynote gxd_assaynote__assay_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_assaynote
    ADD CONSTRAINT gxd_assaynote__assay_key_fkey FOREIGN KEY (_assay_key) REFERENCES mgd.gxd_assay(_assay_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_expression gxd_expression__assay_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_expression
    ADD CONSTRAINT gxd_expression__assay_key_fkey FOREIGN KEY (_assay_key) REFERENCES mgd.gxd_assay(_assay_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_expression gxd_expression__assaytype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_expression
    ADD CONSTRAINT gxd_expression__assaytype_key_fkey FOREIGN KEY (_assaytype_key) REFERENCES mgd.gxd_assaytype(_assaytype_key) DEFERRABLE;


--
-- Name: gxd_expression gxd_expression__celltype_term_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_expression
    ADD CONSTRAINT gxd_expression__celltype_term_key_fkey FOREIGN KEY (_celltype_term_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_expression gxd_expression__emapa_term_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_expression
    ADD CONSTRAINT gxd_expression__emapa_term_key_fkey FOREIGN KEY (_emapa_term_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_expression gxd_expression__gellane_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_expression
    ADD CONSTRAINT gxd_expression__gellane_key_fkey FOREIGN KEY (_gellane_key) REFERENCES mgd.gxd_gellane(_gellane_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_expression gxd_expression__genotype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_expression
    ADD CONSTRAINT gxd_expression__genotype_key_fkey FOREIGN KEY (_genotype_key) REFERENCES mgd.gxd_genotype(_genotype_key) DEFERRABLE;


--
-- Name: gxd_expression gxd_expression__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_expression
    ADD CONSTRAINT gxd_expression__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- Name: gxd_expression gxd_expression__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_expression
    ADD CONSTRAINT gxd_expression__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_expression gxd_expression__specimen_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_expression
    ADD CONSTRAINT gxd_expression__specimen_key_fkey FOREIGN KEY (_specimen_key) REFERENCES mgd.gxd_specimen(_specimen_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_expression gxd_expression__stage_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_expression
    ADD CONSTRAINT gxd_expression__stage_key_fkey FOREIGN KEY (_stage_key) REFERENCES mgd.gxd_theilerstage(_stage_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_gelband gxd_gelband__gellane_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_gelband
    ADD CONSTRAINT gxd_gelband__gellane_key_fkey FOREIGN KEY (_gellane_key) REFERENCES mgd.gxd_gellane(_gellane_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_gelband gxd_gelband__gelrow_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_gelband
    ADD CONSTRAINT gxd_gelband__gelrow_key_fkey FOREIGN KEY (_gelrow_key) REFERENCES mgd.gxd_gelrow(_gelrow_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_gelband gxd_gelband__strength_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_gelband
    ADD CONSTRAINT gxd_gelband__strength_key_fkey FOREIGN KEY (_strength_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_gellane gxd_gellane__assay_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_gellane
    ADD CONSTRAINT gxd_gellane__assay_key_fkey FOREIGN KEY (_assay_key) REFERENCES mgd.gxd_assay(_assay_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_gellane gxd_gellane__gelcontrol_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_gellane
    ADD CONSTRAINT gxd_gellane__gelcontrol_key_fkey FOREIGN KEY (_gelcontrol_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_gellane gxd_gellane__gelrnatype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_gellane
    ADD CONSTRAINT gxd_gellane__gelrnatype_key_fkey FOREIGN KEY (_gelrnatype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_gellane gxd_gellane__genotype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_gellane
    ADD CONSTRAINT gxd_gellane__genotype_key_fkey FOREIGN KEY (_genotype_key) REFERENCES mgd.gxd_genotype(_genotype_key) DEFERRABLE;


--
-- Name: gxd_gellanestructure gxd_gellanestructure__emapa_term_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_gellanestructure
    ADD CONSTRAINT gxd_gellanestructure__emapa_term_key_fkey FOREIGN KEY (_emapa_term_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_gellanestructure gxd_gellanestructure__gellane_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_gellanestructure
    ADD CONSTRAINT gxd_gellanestructure__gellane_key_fkey FOREIGN KEY (_gellane_key) REFERENCES mgd.gxd_gellane(_gellane_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_gellanestructure gxd_gellanestructure__stage_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_gellanestructure
    ADD CONSTRAINT gxd_gellanestructure__stage_key_fkey FOREIGN KEY (_stage_key) REFERENCES mgd.gxd_theilerstage(_stage_key) DEFERRABLE;


--
-- Name: gxd_gelrow gxd_gelrow__assay_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_gelrow
    ADD CONSTRAINT gxd_gelrow__assay_key_fkey FOREIGN KEY (_assay_key) REFERENCES mgd.gxd_assay(_assay_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_gelrow gxd_gelrow__gelunits_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_gelrow
    ADD CONSTRAINT gxd_gelrow__gelunits_key_fkey FOREIGN KEY (_gelunits_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_genotype gxd_genotype__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_genotype
    ADD CONSTRAINT gxd_genotype__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_genotype gxd_genotype__existsas_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_genotype
    ADD CONSTRAINT gxd_genotype__existsas_key_fkey FOREIGN KEY (_existsas_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_genotype gxd_genotype__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_genotype
    ADD CONSTRAINT gxd_genotype__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_genotype gxd_genotype__strain_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_genotype
    ADD CONSTRAINT gxd_genotype__strain_key_fkey FOREIGN KEY (_strain_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


--
-- Name: gxd_htexperiment gxd_htexperiment__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htexperiment
    ADD CONSTRAINT gxd_htexperiment__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_htexperiment gxd_htexperiment__curationstate_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htexperiment
    ADD CONSTRAINT gxd_htexperiment__curationstate_key_fkey FOREIGN KEY (_curationstate_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_htexperiment gxd_htexperiment__evaluatedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htexperiment
    ADD CONSTRAINT gxd_htexperiment__evaluatedby_key_fkey FOREIGN KEY (_evaluatedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_htexperiment gxd_htexperiment__evaluationstate_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htexperiment
    ADD CONSTRAINT gxd_htexperiment__evaluationstate_key_fkey FOREIGN KEY (_evaluationstate_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_htexperiment gxd_htexperiment__experimenttype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htexperiment
    ADD CONSTRAINT gxd_htexperiment__experimenttype_key_fkey FOREIGN KEY (_experimenttype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_htexperiment gxd_htexperiment__initialcuratedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htexperiment
    ADD CONSTRAINT gxd_htexperiment__initialcuratedby_key_fkey FOREIGN KEY (_initialcuratedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_htexperiment gxd_htexperiment__lastcuratedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htexperiment
    ADD CONSTRAINT gxd_htexperiment__lastcuratedby_key_fkey FOREIGN KEY (_lastcuratedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_htexperiment gxd_htexperiment__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htexperiment
    ADD CONSTRAINT gxd_htexperiment__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_htexperiment gxd_htexperiment__source_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htexperiment
    ADD CONSTRAINT gxd_htexperiment__source_key_fkey FOREIGN KEY (_source_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_htexperiment gxd_htexperiment__studytype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htexperiment
    ADD CONSTRAINT gxd_htexperiment__studytype_key_fkey FOREIGN KEY (_studytype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_htexperimentvariable gxd_htexperimentvariable__experiment_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htexperimentvariable
    ADD CONSTRAINT gxd_htexperimentvariable__experiment_key_fkey FOREIGN KEY (_experiment_key) REFERENCES mgd.gxd_htexperiment(_experiment_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_htexperimentvariable gxd_htexperimentvariable__term_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htexperimentvariable
    ADD CONSTRAINT gxd_htexperimentvariable__term_key_fkey FOREIGN KEY (_term_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_htrawsample gxd_htrawsample__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htrawsample
    ADD CONSTRAINT gxd_htrawsample__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_htrawsample gxd_htrawsample__experiment_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htrawsample
    ADD CONSTRAINT gxd_htrawsample__experiment_key_fkey FOREIGN KEY (_experiment_key) REFERENCES mgd.gxd_htexperiment(_experiment_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_htrawsample gxd_htrawsample__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htrawsample
    ADD CONSTRAINT gxd_htrawsample__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_htsample gxd_htsample__celltype_term_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample
    ADD CONSTRAINT gxd_htsample__celltype_term_key_fkey FOREIGN KEY (_celltype_term_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_htsample gxd_htsample__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample
    ADD CONSTRAINT gxd_htsample__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_htsample gxd_htsample__emapa_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample
    ADD CONSTRAINT gxd_htsample__emapa_key_fkey FOREIGN KEY (_emapa_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_htsample gxd_htsample__experiment_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample
    ADD CONSTRAINT gxd_htsample__experiment_key_fkey FOREIGN KEY (_experiment_key) REFERENCES mgd.gxd_htexperiment(_experiment_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_htsample gxd_htsample__genotype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample
    ADD CONSTRAINT gxd_htsample__genotype_key_fkey FOREIGN KEY (_genotype_key) REFERENCES mgd.gxd_genotype(_genotype_key) DEFERRABLE;


--
-- Name: gxd_htsample gxd_htsample__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample
    ADD CONSTRAINT gxd_htsample__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_htsample gxd_htsample__organism_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample
    ADD CONSTRAINT gxd_htsample__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


--
-- Name: gxd_htsample gxd_htsample__relevance_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample
    ADD CONSTRAINT gxd_htsample__relevance_key_fkey FOREIGN KEY (_relevance_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_htsample gxd_htsample__sex_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample
    ADD CONSTRAINT gxd_htsample__sex_key_fkey FOREIGN KEY (_sex_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_htsample gxd_htsample__stage_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample
    ADD CONSTRAINT gxd_htsample__stage_key_fkey FOREIGN KEY (_stage_key) REFERENCES mgd.gxd_theilerstage(_stage_key) DEFERRABLE;


--
-- Name: gxd_htsample_rnaseq gxd_htsample_rnaseq__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseq
    ADD CONSTRAINT gxd_htsample_rnaseq__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_htsample_rnaseq gxd_htsample_rnaseq__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseq
    ADD CONSTRAINT gxd_htsample_rnaseq__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- Name: gxd_htsample_rnaseq gxd_htsample_rnaseq__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseq
    ADD CONSTRAINT gxd_htsample_rnaseq__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_htsample_rnaseq gxd_htsample_rnaseq__rnaseqcombined_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseq
    ADD CONSTRAINT gxd_htsample_rnaseq__rnaseqcombined_key_fkey FOREIGN KEY (_rnaseqcombined_key) REFERENCES mgd.gxd_htsample_rnaseqcombined(_rnaseqcombined_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_htsample_rnaseq gxd_htsample_rnaseq__sample_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseq
    ADD CONSTRAINT gxd_htsample_rnaseq__sample_key_fkey FOREIGN KEY (_sample_key) REFERENCES mgd.gxd_htsample(_sample_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_htsample_rnaseqcombined gxd_htsample_rnaseqcombined__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseqcombined
    ADD CONSTRAINT gxd_htsample_rnaseqcombined__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_htsample_rnaseqcombined gxd_htsample_rnaseqcombined__level_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseqcombined
    ADD CONSTRAINT gxd_htsample_rnaseqcombined__level_key_fkey FOREIGN KEY (_level_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_htsample_rnaseqcombined gxd_htsample_rnaseqcombined__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseqcombined
    ADD CONSTRAINT gxd_htsample_rnaseqcombined__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- Name: gxd_htsample_rnaseqcombined gxd_htsample_rnaseqcombined__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseqcombined
    ADD CONSTRAINT gxd_htsample_rnaseqcombined__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_htsample_rnaseqset gxd_htsample_rnaseqset__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset
    ADD CONSTRAINT gxd_htsample_rnaseqset__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_htsample_rnaseqset gxd_htsample_rnaseqset__emapa_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset
    ADD CONSTRAINT gxd_htsample_rnaseqset__emapa_key_fkey FOREIGN KEY (_emapa_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_htsample_rnaseqset gxd_htsample_rnaseqset__experiment_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset
    ADD CONSTRAINT gxd_htsample_rnaseqset__experiment_key_fkey FOREIGN KEY (_experiment_key) REFERENCES mgd.gxd_htexperiment(_experiment_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_htsample_rnaseqset gxd_htsample_rnaseqset__genotype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset
    ADD CONSTRAINT gxd_htsample_rnaseqset__genotype_key_fkey FOREIGN KEY (_genotype_key) REFERENCES mgd.gxd_genotype(_genotype_key) DEFERRABLE;


--
-- Name: gxd_htsample_rnaseqset gxd_htsample_rnaseqset__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset
    ADD CONSTRAINT gxd_htsample_rnaseqset__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_htsample_rnaseqset gxd_htsample_rnaseqset__organism_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset
    ADD CONSTRAINT gxd_htsample_rnaseqset__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


--
-- Name: gxd_htsample_rnaseqset gxd_htsample_rnaseqset__sex_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset
    ADD CONSTRAINT gxd_htsample_rnaseqset__sex_key_fkey FOREIGN KEY (_sex_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_htsample_rnaseqset gxd_htsample_rnaseqset__stage_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset
    ADD CONSTRAINT gxd_htsample_rnaseqset__stage_key_fkey FOREIGN KEY (_stage_key) REFERENCES mgd.gxd_theilerstage(_stage_key) DEFERRABLE;


--
-- Name: gxd_htsample_rnaseqset_cache gxd_htsample_rnaseqset_cache__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset_cache
    ADD CONSTRAINT gxd_htsample_rnaseqset_cache__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_htsample_rnaseqset_cache gxd_htsample_rnaseqset_cache__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset_cache
    ADD CONSTRAINT gxd_htsample_rnaseqset_cache__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_htsample_rnaseqset_cache gxd_htsample_rnaseqset_cache__rnaseqcombined_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset_cache
    ADD CONSTRAINT gxd_htsample_rnaseqset_cache__rnaseqcombined_key_fkey FOREIGN KEY (_rnaseqcombined_key) REFERENCES mgd.gxd_htsample_rnaseqcombined(_rnaseqcombined_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_htsample_rnaseqset_cache gxd_htsample_rnaseqset_cache__rnaseqset_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseqset_cache
    ADD CONSTRAINT gxd_htsample_rnaseqset_cache__rnaseqset_key_fkey FOREIGN KEY (_rnaseqset_key) REFERENCES mgd.gxd_htsample_rnaseqset(_rnaseqset_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_htsample_rnaseqsetmember gxd_htsample_rnaseqsetmember__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseqsetmember
    ADD CONSTRAINT gxd_htsample_rnaseqsetmember__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_htsample_rnaseqsetmember gxd_htsample_rnaseqsetmember__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseqsetmember
    ADD CONSTRAINT gxd_htsample_rnaseqsetmember__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_htsample_rnaseqsetmember gxd_htsample_rnaseqsetmember__rnaseqset_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseqsetmember
    ADD CONSTRAINT gxd_htsample_rnaseqsetmember__rnaseqset_key_fkey FOREIGN KEY (_rnaseqset_key) REFERENCES mgd.gxd_htsample_rnaseqset(_rnaseqset_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_htsample_rnaseqsetmember gxd_htsample_rnaseqsetmember__sample_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_htsample_rnaseqsetmember
    ADD CONSTRAINT gxd_htsample_rnaseqsetmember__sample_key_fkey FOREIGN KEY (_sample_key) REFERENCES mgd.gxd_htsample(_sample_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_index gxd_index__conditionalmutants_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_index
    ADD CONSTRAINT gxd_index__conditionalmutants_key_fkey FOREIGN KEY (_conditionalmutants_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_index gxd_index__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_index
    ADD CONSTRAINT gxd_index__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_index gxd_index__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_index
    ADD CONSTRAINT gxd_index__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- Name: gxd_index gxd_index__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_index
    ADD CONSTRAINT gxd_index__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_index gxd_index__priority_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_index
    ADD CONSTRAINT gxd_index__priority_key_fkey FOREIGN KEY (_priority_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_index gxd_index__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_index
    ADD CONSTRAINT gxd_index__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


--
-- Name: gxd_index_stages gxd_index_stages__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_index_stages
    ADD CONSTRAINT gxd_index_stages__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_index_stages gxd_index_stages__index_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_index_stages
    ADD CONSTRAINT gxd_index_stages__index_key_fkey FOREIGN KEY (_index_key) REFERENCES mgd.gxd_index(_index_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_index_stages gxd_index_stages__indexassay_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_index_stages
    ADD CONSTRAINT gxd_index_stages__indexassay_key_fkey FOREIGN KEY (_indexassay_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_index_stages gxd_index_stages__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_index_stages
    ADD CONSTRAINT gxd_index_stages__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: gxd_index_stages gxd_index_stages__stageid_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_index_stages
    ADD CONSTRAINT gxd_index_stages__stageid_key_fkey FOREIGN KEY (_stageid_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_insituresult gxd_insituresult__pattern_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_insituresult
    ADD CONSTRAINT gxd_insituresult__pattern_key_fkey FOREIGN KEY (_pattern_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_insituresult gxd_insituresult__specimen_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_insituresult
    ADD CONSTRAINT gxd_insituresult__specimen_key_fkey FOREIGN KEY (_specimen_key) REFERENCES mgd.gxd_specimen(_specimen_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_insituresult gxd_insituresult__strength_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_insituresult
    ADD CONSTRAINT gxd_insituresult__strength_key_fkey FOREIGN KEY (_strength_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_insituresultimage gxd_insituresultimage__imagepane_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_insituresultimage
    ADD CONSTRAINT gxd_insituresultimage__imagepane_key_fkey FOREIGN KEY (_imagepane_key) REFERENCES mgd.img_imagepane(_imagepane_key) DEFERRABLE;


--
-- Name: gxd_insituresultimage gxd_insituresultimage__result_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_insituresultimage
    ADD CONSTRAINT gxd_insituresultimage__result_key_fkey FOREIGN KEY (_result_key) REFERENCES mgd.gxd_insituresult(_result_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_isresultcelltype gxd_isresultcelltype__celltype_term_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_isresultcelltype
    ADD CONSTRAINT gxd_isresultcelltype__celltype_term_key_fkey FOREIGN KEY (_celltype_term_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_isresultcelltype gxd_isresultcelltype__result_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_isresultcelltype
    ADD CONSTRAINT gxd_isresultcelltype__result_key_fkey FOREIGN KEY (_result_key) REFERENCES mgd.gxd_insituresult(_result_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_isresultstructure gxd_isresultstructure__emapa_term_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_isresultstructure
    ADD CONSTRAINT gxd_isresultstructure__emapa_term_key_fkey FOREIGN KEY (_emapa_term_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_isresultstructure gxd_isresultstructure__result_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_isresultstructure
    ADD CONSTRAINT gxd_isresultstructure__result_key_fkey FOREIGN KEY (_result_key) REFERENCES mgd.gxd_insituresult(_result_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_isresultstructure gxd_isresultstructure__stage_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_isresultstructure
    ADD CONSTRAINT gxd_isresultstructure__stage_key_fkey FOREIGN KEY (_stage_key) REFERENCES mgd.gxd_theilerstage(_stage_key) DEFERRABLE;


--
-- Name: gxd_probeprep gxd_probeprep__label_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_probeprep
    ADD CONSTRAINT gxd_probeprep__label_key_fkey FOREIGN KEY (_label_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_probeprep gxd_probeprep__probe_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_probeprep
    ADD CONSTRAINT gxd_probeprep__probe_key_fkey FOREIGN KEY (_probe_key) REFERENCES mgd.prb_probe(_probe_key) DEFERRABLE;


--
-- Name: gxd_probeprep gxd_probeprep__sense_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_probeprep
    ADD CONSTRAINT gxd_probeprep__sense_key_fkey FOREIGN KEY (_sense_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_probeprep gxd_probeprep__visualization_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_probeprep
    ADD CONSTRAINT gxd_probeprep__visualization_key_fkey FOREIGN KEY (_visualization_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_specimen gxd_specimen__assay_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_specimen
    ADD CONSTRAINT gxd_specimen__assay_key_fkey FOREIGN KEY (_assay_key) REFERENCES mgd.gxd_assay(_assay_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: gxd_specimen gxd_specimen__embedding_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_specimen
    ADD CONSTRAINT gxd_specimen__embedding_key_fkey FOREIGN KEY (_embedding_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_specimen gxd_specimen__fixation_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_specimen
    ADD CONSTRAINT gxd_specimen__fixation_key_fkey FOREIGN KEY (_fixation_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: gxd_specimen gxd_specimen__genotype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.gxd_specimen
    ADD CONSTRAINT gxd_specimen__genotype_key_fkey FOREIGN KEY (_genotype_key) REFERENCES mgd.gxd_genotype(_genotype_key) DEFERRABLE;


--
-- Name: img_image img_image__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.img_image
    ADD CONSTRAINT img_image__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: img_image img_image__imageclass_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.img_image
    ADD CONSTRAINT img_image__imageclass_key_fkey FOREIGN KEY (_imageclass_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: img_image img_image__imagetype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.img_image
    ADD CONSTRAINT img_image__imagetype_key_fkey FOREIGN KEY (_imagetype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: img_image img_image__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.img_image
    ADD CONSTRAINT img_image__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: img_image img_image__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.img_image
    ADD CONSTRAINT img_image__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


--
-- Name: img_image img_image__thumbnailimage_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.img_image
    ADD CONSTRAINT img_image__thumbnailimage_key_fkey FOREIGN KEY (_thumbnailimage_key) REFERENCES mgd.img_image(_image_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: img_imagepane img_imagepane__image_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.img_imagepane
    ADD CONSTRAINT img_imagepane__image_key_fkey FOREIGN KEY (_image_key) REFERENCES mgd.img_image(_image_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: img_imagepane_assoc img_imagepane_assoc__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.img_imagepane_assoc
    ADD CONSTRAINT img_imagepane_assoc__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: img_imagepane_assoc img_imagepane_assoc__imagepane_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.img_imagepane_assoc
    ADD CONSTRAINT img_imagepane_assoc__imagepane_key_fkey FOREIGN KEY (_imagepane_key) REFERENCES mgd.img_imagepane(_imagepane_key) DEFERRABLE;


--
-- Name: img_imagepane_assoc img_imagepane_assoc__mgitype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.img_imagepane_assoc
    ADD CONSTRAINT img_imagepane_assoc__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


--
-- Name: img_imagepane_assoc img_imagepane_assoc__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.img_imagepane_assoc
    ADD CONSTRAINT img_imagepane_assoc__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: map_coord_collection map_coord_collection__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.map_coord_collection
    ADD CONSTRAINT map_coord_collection__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: map_coord_collection map_coord_collection__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.map_coord_collection
    ADD CONSTRAINT map_coord_collection__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: map_coord_feature map_coord_feature__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.map_coord_feature
    ADD CONSTRAINT map_coord_feature__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: map_coord_feature map_coord_feature__map_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.map_coord_feature
    ADD CONSTRAINT map_coord_feature__map_key_fkey FOREIGN KEY (_map_key) REFERENCES mgd.map_coordinate(_map_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: map_coord_feature map_coord_feature__mgitype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.map_coord_feature
    ADD CONSTRAINT map_coord_feature__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


--
-- Name: map_coord_feature map_coord_feature__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.map_coord_feature
    ADD CONSTRAINT map_coord_feature__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: map_coordinate map_coordinate__collection_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.map_coordinate
    ADD CONSTRAINT map_coordinate__collection_key_fkey FOREIGN KEY (_collection_key) REFERENCES mgd.map_coord_collection(_collection_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: map_coordinate map_coordinate__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.map_coordinate
    ADD CONSTRAINT map_coordinate__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: map_coordinate map_coordinate__maptype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.map_coordinate
    ADD CONSTRAINT map_coordinate__maptype_key_fkey FOREIGN KEY (_maptype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: map_coordinate map_coordinate__mgitype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.map_coordinate
    ADD CONSTRAINT map_coordinate__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


--
-- Name: map_coordinate map_coordinate__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.map_coordinate
    ADD CONSTRAINT map_coordinate__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: map_coordinate map_coordinate__units_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.map_coordinate
    ADD CONSTRAINT map_coordinate__units_key_fkey FOREIGN KEY (_units_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: mgi_keyvalue mgi_keyvalue__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_keyvalue
    ADD CONSTRAINT mgi_keyvalue__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_keyvalue mgi_keyvalue__mgitype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_keyvalue
    ADD CONSTRAINT mgi_keyvalue__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


--
-- Name: mgi_keyvalue mgi_keyvalue__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_keyvalue
    ADD CONSTRAINT mgi_keyvalue__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_note mgi_note__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_note
    ADD CONSTRAINT mgi_note__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_note mgi_note__mgitype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_note
    ADD CONSTRAINT mgi_note__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


--
-- Name: mgi_note mgi_note__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_note
    ADD CONSTRAINT mgi_note__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_note mgi_note__notetype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_note
    ADD CONSTRAINT mgi_note__notetype_key_fkey FOREIGN KEY (_notetype_key) REFERENCES mgd.mgi_notetype(_notetype_key) DEFERRABLE;


--
-- Name: mgi_notetype mgi_notetype__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_notetype
    ADD CONSTRAINT mgi_notetype__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_notetype mgi_notetype__mgitype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_notetype
    ADD CONSTRAINT mgi_notetype__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


--
-- Name: mgi_notetype mgi_notetype__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_notetype
    ADD CONSTRAINT mgi_notetype__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_organism mgi_organism__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_organism
    ADD CONSTRAINT mgi_organism__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_organism mgi_organism__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_organism
    ADD CONSTRAINT mgi_organism__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_organism_mgitype mgi_organism_mgitype__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_organism_mgitype
    ADD CONSTRAINT mgi_organism_mgitype__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_organism_mgitype mgi_organism_mgitype__mgitype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_organism_mgitype
    ADD CONSTRAINT mgi_organism_mgitype__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


--
-- Name: mgi_organism_mgitype mgi_organism_mgitype__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_organism_mgitype
    ADD CONSTRAINT mgi_organism_mgitype__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_organism_mgitype mgi_organism_mgitype__organism_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_organism_mgitype
    ADD CONSTRAINT mgi_organism_mgitype__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mgi_property mgi_property__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_property
    ADD CONSTRAINT mgi_property__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_property mgi_property__mgitype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_property
    ADD CONSTRAINT mgi_property__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


--
-- Name: mgi_property mgi_property__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_property
    ADD CONSTRAINT mgi_property__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_property mgi_property__propertyterm_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_property
    ADD CONSTRAINT mgi_property__propertyterm_key_fkey FOREIGN KEY (_propertyterm_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: mgi_property mgi_property__propertytype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_property
    ADD CONSTRAINT mgi_property__propertytype_key_fkey FOREIGN KEY (_propertytype_key) REFERENCES mgd.mgi_propertytype(_propertytype_key) DEFERRABLE;


--
-- Name: mgi_propertytype mgi_propertytype__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_propertytype
    ADD CONSTRAINT mgi_propertytype__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_propertytype mgi_propertytype__mgitype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_propertytype
    ADD CONSTRAINT mgi_propertytype__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


--
-- Name: mgi_propertytype mgi_propertytype__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_propertytype
    ADD CONSTRAINT mgi_propertytype__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_propertytype mgi_propertytype__vocab_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_propertytype
    ADD CONSTRAINT mgi_propertytype__vocab_key_fkey FOREIGN KEY (_vocab_key) REFERENCES mgd.voc_vocab(_vocab_key) DEFERRABLE;


--
-- Name: mgi_refassoctype mgi_refassoctype__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_refassoctype
    ADD CONSTRAINT mgi_refassoctype__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_refassoctype mgi_refassoctype__mgitype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_refassoctype
    ADD CONSTRAINT mgi_refassoctype__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


--
-- Name: mgi_refassoctype mgi_refassoctype__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_refassoctype
    ADD CONSTRAINT mgi_refassoctype__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_reference_assoc mgi_reference_assoc__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_reference_assoc
    ADD CONSTRAINT mgi_reference_assoc__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_reference_assoc mgi_reference_assoc__mgitype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_reference_assoc
    ADD CONSTRAINT mgi_reference_assoc__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


--
-- Name: mgi_reference_assoc mgi_reference_assoc__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_reference_assoc
    ADD CONSTRAINT mgi_reference_assoc__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_reference_assoc mgi_reference_assoc__refassoctype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_reference_assoc
    ADD CONSTRAINT mgi_reference_assoc__refassoctype_key_fkey FOREIGN KEY (_refassoctype_key) REFERENCES mgd.mgi_refassoctype(_refassoctype_key) DEFERRABLE;


--
-- Name: mgi_reference_assoc mgi_reference_assoc__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_reference_assoc
    ADD CONSTRAINT mgi_reference_assoc__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


--
-- Name: mgi_relationship mgi_relationship__category_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_relationship
    ADD CONSTRAINT mgi_relationship__category_key_fkey FOREIGN KEY (_category_key) REFERENCES mgd.mgi_relationship_category(_category_key) DEFERRABLE;


--
-- Name: mgi_relationship mgi_relationship__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_relationship
    ADD CONSTRAINT mgi_relationship__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_relationship mgi_relationship__evidence_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_relationship
    ADD CONSTRAINT mgi_relationship__evidence_key_fkey FOREIGN KEY (_evidence_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: mgi_relationship mgi_relationship__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_relationship
    ADD CONSTRAINT mgi_relationship__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_relationship mgi_relationship__qualifier_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_relationship
    ADD CONSTRAINT mgi_relationship__qualifier_key_fkey FOREIGN KEY (_qualifier_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: mgi_relationship mgi_relationship__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_relationship
    ADD CONSTRAINT mgi_relationship__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


--
-- Name: mgi_relationship mgi_relationship__relationshipterm_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_relationship
    ADD CONSTRAINT mgi_relationship__relationshipterm_key_fkey FOREIGN KEY (_relationshipterm_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: mgi_relationship_category mgi_relationship_category__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_relationship_category
    ADD CONSTRAINT mgi_relationship_category__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_relationship_category mgi_relationship_category__evidencevocab_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_relationship_category
    ADD CONSTRAINT mgi_relationship_category__evidencevocab_key_fkey FOREIGN KEY (_evidencevocab_key) REFERENCES mgd.voc_vocab(_vocab_key) DEFERRABLE;


--
-- Name: mgi_relationship_category mgi_relationship_category__mgitype_key_1_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_relationship_category
    ADD CONSTRAINT mgi_relationship_category__mgitype_key_1_fkey FOREIGN KEY (_mgitype_key_1) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


--
-- Name: mgi_relationship_category mgi_relationship_category__mgitype_key_2_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_relationship_category
    ADD CONSTRAINT mgi_relationship_category__mgitype_key_2_fkey FOREIGN KEY (_mgitype_key_2) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


--
-- Name: mgi_relationship_category mgi_relationship_category__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_relationship_category
    ADD CONSTRAINT mgi_relationship_category__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_relationship_category mgi_relationship_category__qualifiervocab_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_relationship_category
    ADD CONSTRAINT mgi_relationship_category__qualifiervocab_key_fkey FOREIGN KEY (_qualifiervocab_key) REFERENCES mgd.voc_vocab(_vocab_key) DEFERRABLE;


--
-- Name: mgi_relationship_category mgi_relationship_category__relationshipdag_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_relationship_category
    ADD CONSTRAINT mgi_relationship_category__relationshipdag_key_fkey FOREIGN KEY (_relationshipdag_key) REFERENCES mgd.dag_dag(_dag_key) DEFERRABLE;


--
-- Name: mgi_relationship_category mgi_relationship_category__relationshipvocab_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_relationship_category
    ADD CONSTRAINT mgi_relationship_category__relationshipvocab_key_fkey FOREIGN KEY (_relationshipvocab_key) REFERENCES mgd.voc_vocab(_vocab_key) DEFERRABLE;


--
-- Name: mgi_relationship_property mgi_relationship_property__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_relationship_property
    ADD CONSTRAINT mgi_relationship_property__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_relationship_property mgi_relationship_property__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_relationship_property
    ADD CONSTRAINT mgi_relationship_property__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_relationship_property mgi_relationship_property__propertyname_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_relationship_property
    ADD CONSTRAINT mgi_relationship_property__propertyname_key_fkey FOREIGN KEY (_propertyname_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: mgi_relationship_property mgi_relationship_property__relationship_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_relationship_property
    ADD CONSTRAINT mgi_relationship_property__relationship_key_fkey FOREIGN KEY (_relationship_key) REFERENCES mgd.mgi_relationship(_relationship_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mgi_set mgi_set__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_set
    ADD CONSTRAINT mgi_set__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_set mgi_set__mgitype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_set
    ADD CONSTRAINT mgi_set__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


--
-- Name: mgi_set mgi_set__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_set
    ADD CONSTRAINT mgi_set__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_setmember mgi_setmember__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_setmember
    ADD CONSTRAINT mgi_setmember__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_setmember mgi_setmember__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_setmember
    ADD CONSTRAINT mgi_setmember__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_setmember mgi_setmember__set_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_setmember
    ADD CONSTRAINT mgi_setmember__set_key_fkey FOREIGN KEY (_set_key) REFERENCES mgd.mgi_set(_set_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mgi_setmember_emapa mgi_setmember_emapa__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_setmember_emapa
    ADD CONSTRAINT mgi_setmember_emapa__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_setmember_emapa mgi_setmember_emapa__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_setmember_emapa
    ADD CONSTRAINT mgi_setmember_emapa__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_setmember_emapa mgi_setmember_emapa__setmember_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_setmember_emapa
    ADD CONSTRAINT mgi_setmember_emapa__setmember_key_fkey FOREIGN KEY (_setmember_key) REFERENCES mgd.mgi_setmember(_setmember_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mgi_setmember_emapa mgi_setmember_emapa__stage_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_setmember_emapa
    ADD CONSTRAINT mgi_setmember_emapa__stage_key_fkey FOREIGN KEY (_stage_key) REFERENCES mgd.gxd_theilerstage(_stage_key) DEFERRABLE;


--
-- Name: mgi_synonym mgi_synonym__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_synonym
    ADD CONSTRAINT mgi_synonym__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_synonym mgi_synonym__mgitype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_synonym
    ADD CONSTRAINT mgi_synonym__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


--
-- Name: mgi_synonym mgi_synonym__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_synonym
    ADD CONSTRAINT mgi_synonym__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_synonym mgi_synonym__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_synonym
    ADD CONSTRAINT mgi_synonym__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


--
-- Name: mgi_synonym mgi_synonym__synonymtype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_synonym
    ADD CONSTRAINT mgi_synonym__synonymtype_key_fkey FOREIGN KEY (_synonymtype_key) REFERENCES mgd.mgi_synonymtype(_synonymtype_key) DEFERRABLE;


--
-- Name: mgi_synonymtype mgi_synonymtype__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_synonymtype
    ADD CONSTRAINT mgi_synonymtype__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_synonymtype mgi_synonymtype__mgitype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_synonymtype
    ADD CONSTRAINT mgi_synonymtype__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


--
-- Name: mgi_synonymtype mgi_synonymtype__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_synonymtype
    ADD CONSTRAINT mgi_synonymtype__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_synonymtype mgi_synonymtype__organism_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_synonymtype
    ADD CONSTRAINT mgi_synonymtype__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


--
-- Name: mgi_translation mgi_translation__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_translation
    ADD CONSTRAINT mgi_translation__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_translation mgi_translation__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_translation
    ADD CONSTRAINT mgi_translation__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_translation mgi_translation__translationtype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_translation
    ADD CONSTRAINT mgi_translation__translationtype_key_fkey FOREIGN KEY (_translationtype_key) REFERENCES mgd.mgi_translationtype(_translationtype_key) DEFERRABLE;


--
-- Name: mgi_translationtype mgi_translationtype__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_translationtype
    ADD CONSTRAINT mgi_translationtype__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_translationtype mgi_translationtype__mgitype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_translationtype
    ADD CONSTRAINT mgi_translationtype__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


--
-- Name: mgi_translationtype mgi_translationtype__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_translationtype
    ADD CONSTRAINT mgi_translationtype__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_translationtype mgi_translationtype__vocab_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_translationtype
    ADD CONSTRAINT mgi_translationtype__vocab_key_fkey FOREIGN KEY (_vocab_key) REFERENCES mgd.voc_vocab(_vocab_key) DEFERRABLE;


--
-- Name: mgi_user mgi_user__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_user
    ADD CONSTRAINT mgi_user__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_user mgi_user__group_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_user
    ADD CONSTRAINT mgi_user__group_key_fkey FOREIGN KEY (_group_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: mgi_user mgi_user__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_user
    ADD CONSTRAINT mgi_user__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mgi_user mgi_user__userstatus_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_user
    ADD CONSTRAINT mgi_user__userstatus_key_fkey FOREIGN KEY (_userstatus_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: mgi_user mgi_user__usertype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mgi_user
    ADD CONSTRAINT mgi_user__usertype_key_fkey FOREIGN KEY (_usertype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: mld_concordance mld_concordance__expt_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_concordance
    ADD CONSTRAINT mld_concordance__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mld_concordance mld_concordance__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_concordance
    ADD CONSTRAINT mld_concordance__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- Name: mld_contig mld_contig__expt_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_contig
    ADD CONSTRAINT mld_contig__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mld_contigprobe mld_contigprobe__contig_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_contigprobe
    ADD CONSTRAINT mld_contigprobe__contig_key_fkey FOREIGN KEY (_contig_key) REFERENCES mgd.mld_contig(_contig_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mld_contigprobe mld_contigprobe__probe_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_contigprobe
    ADD CONSTRAINT mld_contigprobe__probe_key_fkey FOREIGN KEY (_probe_key) REFERENCES mgd.prb_probe(_probe_key) DEFERRABLE;


--
-- Name: mld_expt_marker mld_expt_marker__allele_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_expt_marker
    ADD CONSTRAINT mld_expt_marker__allele_key_fkey FOREIGN KEY (_allele_key) REFERENCES mgd.all_allele(_allele_key) DEFERRABLE;


--
-- Name: mld_expt_marker mld_expt_marker__assay_type_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_expt_marker
    ADD CONSTRAINT mld_expt_marker__assay_type_key_fkey FOREIGN KEY (_assay_type_key) REFERENCES mgd.mld_assay_types(_assay_type_key) DEFERRABLE;


--
-- Name: mld_expt_marker mld_expt_marker__expt_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_expt_marker
    ADD CONSTRAINT mld_expt_marker__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mld_expt_marker mld_expt_marker__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_expt_marker
    ADD CONSTRAINT mld_expt_marker__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- Name: mld_expt_notes mld_expt_notes__expt_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_expt_notes
    ADD CONSTRAINT mld_expt_notes__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mld_expts mld_expts__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_expts
    ADD CONSTRAINT mld_expts__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


--
-- Name: mld_fish mld_fish__expt_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_fish
    ADD CONSTRAINT mld_fish__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mld_fish mld_fish__strain_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_fish
    ADD CONSTRAINT mld_fish__strain_key_fkey FOREIGN KEY (_strain_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


--
-- Name: mld_fish_region mld_fish_region__expt_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_fish_region
    ADD CONSTRAINT mld_fish_region__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mld_hit mld_hit__expt_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_hit
    ADD CONSTRAINT mld_hit__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mld_hit mld_hit__probe_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_hit
    ADD CONSTRAINT mld_hit__probe_key_fkey FOREIGN KEY (_probe_key) REFERENCES mgd.prb_probe(_probe_key) DEFERRABLE;


--
-- Name: mld_hit mld_hit__target_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_hit
    ADD CONSTRAINT mld_hit__target_key_fkey FOREIGN KEY (_target_key) REFERENCES mgd.prb_probe(_probe_key) DEFERRABLE;


--
-- Name: mld_hybrid mld_hybrid__expt_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_hybrid
    ADD CONSTRAINT mld_hybrid__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mld_insitu mld_insitu__expt_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_insitu
    ADD CONSTRAINT mld_insitu__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mld_insitu mld_insitu__strain_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_insitu
    ADD CONSTRAINT mld_insitu__strain_key_fkey FOREIGN KEY (_strain_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


--
-- Name: mld_isregion mld_isregion__expt_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_isregion
    ADD CONSTRAINT mld_isregion__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mld_matrix mld_matrix__cross_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_matrix
    ADD CONSTRAINT mld_matrix__cross_key_fkey FOREIGN KEY (_cross_key) REFERENCES mgd.crs_cross(_cross_key) DEFERRABLE;


--
-- Name: mld_matrix mld_matrix__expt_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_matrix
    ADD CONSTRAINT mld_matrix__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mld_mc2point mld_mc2point__expt_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_mc2point
    ADD CONSTRAINT mld_mc2point__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mld_mc2point mld_mc2point__marker_key_1_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_mc2point
    ADD CONSTRAINT mld_mc2point__marker_key_1_fkey FOREIGN KEY (_marker_key_1) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- Name: mld_mc2point mld_mc2point__marker_key_2_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_mc2point
    ADD CONSTRAINT mld_mc2point__marker_key_2_fkey FOREIGN KEY (_marker_key_2) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- Name: mld_mcdatalist mld_mcdatalist__expt_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_mcdatalist
    ADD CONSTRAINT mld_mcdatalist__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mld_notes mld_notes__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_notes
    ADD CONSTRAINT mld_notes__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


--
-- Name: mld_ri2point mld_ri2point__expt_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_ri2point
    ADD CONSTRAINT mld_ri2point__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mld_ri2point mld_ri2point__marker_key_1_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_ri2point
    ADD CONSTRAINT mld_ri2point__marker_key_1_fkey FOREIGN KEY (_marker_key_1) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- Name: mld_ri2point mld_ri2point__marker_key_2_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_ri2point
    ADD CONSTRAINT mld_ri2point__marker_key_2_fkey FOREIGN KEY (_marker_key_2) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- Name: mld_ri mld_ri__expt_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_ri
    ADD CONSTRAINT mld_ri__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mld_ridata mld_ridata__expt_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_ridata
    ADD CONSTRAINT mld_ridata__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mld_ridata mld_ridata__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_ridata
    ADD CONSTRAINT mld_ridata__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- Name: mld_statistics mld_statistics__expt_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_statistics
    ADD CONSTRAINT mld_statistics__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mld_statistics mld_statistics__marker_key_1_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_statistics
    ADD CONSTRAINT mld_statistics__marker_key_1_fkey FOREIGN KEY (_marker_key_1) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- Name: mld_statistics mld_statistics__marker_key_2_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mld_statistics
    ADD CONSTRAINT mld_statistics__marker_key_2_fkey FOREIGN KEY (_marker_key_2) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- Name: mrk_biotypemapping mrk_biotypemapping__biotypeterm_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_biotypemapping
    ADD CONSTRAINT mrk_biotypemapping__biotypeterm_key_fkey FOREIGN KEY (_biotypeterm_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: mrk_biotypemapping mrk_biotypemapping__biotypevocab_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_biotypemapping
    ADD CONSTRAINT mrk_biotypemapping__biotypevocab_key_fkey FOREIGN KEY (_biotypevocab_key) REFERENCES mgd.voc_vocab(_vocab_key) DEFERRABLE;


--
-- Name: mrk_biotypemapping mrk_biotypemapping__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_biotypemapping
    ADD CONSTRAINT mrk_biotypemapping__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mrk_biotypemapping mrk_biotypemapping__marker_type_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_biotypemapping
    ADD CONSTRAINT mrk_biotypemapping__marker_type_key_fkey FOREIGN KEY (_marker_type_key) REFERENCES mgd.mrk_types(_marker_type_key) DEFERRABLE;


--
-- Name: mrk_biotypemapping mrk_biotypemapping__mcvterm_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_biotypemapping
    ADD CONSTRAINT mrk_biotypemapping__mcvterm_key_fkey FOREIGN KEY (_mcvterm_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: mrk_biotypemapping mrk_biotypemapping__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_biotypemapping
    ADD CONSTRAINT mrk_biotypemapping__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mrk_biotypemapping mrk_biotypemapping__primarymcvterm_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_biotypemapping
    ADD CONSTRAINT mrk_biotypemapping__primarymcvterm_key_fkey FOREIGN KEY (_primarymcvterm_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: mrk_chromosome mrk_chromosome__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_chromosome
    ADD CONSTRAINT mrk_chromosome__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mrk_chromosome mrk_chromosome__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_chromosome
    ADD CONSTRAINT mrk_chromosome__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mrk_chromosome mrk_chromosome__organism_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_chromosome
    ADD CONSTRAINT mrk_chromosome__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mrk_cluster mrk_cluster__clustersource_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_cluster
    ADD CONSTRAINT mrk_cluster__clustersource_key_fkey FOREIGN KEY (_clustersource_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: mrk_cluster mrk_cluster__clustertype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_cluster
    ADD CONSTRAINT mrk_cluster__clustertype_key_fkey FOREIGN KEY (_clustertype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: mrk_cluster mrk_cluster__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_cluster
    ADD CONSTRAINT mrk_cluster__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mrk_cluster mrk_cluster__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_cluster
    ADD CONSTRAINT mrk_cluster__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mrk_clustermember mrk_clustermember__cluster_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_clustermember
    ADD CONSTRAINT mrk_clustermember__cluster_key_fkey FOREIGN KEY (_cluster_key) REFERENCES mgd.mrk_cluster(_cluster_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mrk_clustermember mrk_clustermember__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_clustermember
    ADD CONSTRAINT mrk_clustermember__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- Name: mrk_current mrk_current__current_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_current
    ADD CONSTRAINT mrk_current__current_key_fkey FOREIGN KEY (_current_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mrk_current mrk_current__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_current
    ADD CONSTRAINT mrk_current__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mrk_do_cache mrk_do_cache__genotype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_do_cache
    ADD CONSTRAINT mrk_do_cache__genotype_key_fkey FOREIGN KEY (_genotype_key) REFERENCES mgd.gxd_genotype(_genotype_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mrk_do_cache mrk_do_cache__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_do_cache
    ADD CONSTRAINT mrk_do_cache__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mrk_do_cache mrk_do_cache__organism_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_do_cache
    ADD CONSTRAINT mrk_do_cache__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


--
-- Name: mrk_do_cache mrk_do_cache__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_do_cache
    ADD CONSTRAINT mrk_do_cache__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mrk_do_cache mrk_do_cache__term_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_do_cache
    ADD CONSTRAINT mrk_do_cache__term_key_fkey FOREIGN KEY (_term_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mrk_history mrk_history__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_history
    ADD CONSTRAINT mrk_history__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mrk_history mrk_history__history_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_history
    ADD CONSTRAINT mrk_history__history_key_fkey FOREIGN KEY (_history_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mrk_history mrk_history__marker_event_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_history
    ADD CONSTRAINT mrk_history__marker_event_key_fkey FOREIGN KEY (_marker_event_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: mrk_history mrk_history__marker_eventreason_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_history
    ADD CONSTRAINT mrk_history__marker_eventreason_key_fkey FOREIGN KEY (_marker_eventreason_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: mrk_history mrk_history__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_history
    ADD CONSTRAINT mrk_history__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mrk_history mrk_history__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_history
    ADD CONSTRAINT mrk_history__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mrk_history mrk_history__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_history
    ADD CONSTRAINT mrk_history__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


--
-- Name: mrk_label mrk_label__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_label
    ADD CONSTRAINT mrk_label__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mrk_label mrk_label__organism_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_label
    ADD CONSTRAINT mrk_label__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


--
-- Name: mrk_label mrk_label__orthologorganism_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_label
    ADD CONSTRAINT mrk_label__orthologorganism_key_fkey FOREIGN KEY (_orthologorganism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


--
-- Name: mrk_location_cache mrk_location_cache__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_location_cache
    ADD CONSTRAINT mrk_location_cache__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mrk_location_cache mrk_location_cache__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_location_cache
    ADD CONSTRAINT mrk_location_cache__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mrk_location_cache mrk_location_cache__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_location_cache
    ADD CONSTRAINT mrk_location_cache__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mrk_location_cache mrk_location_cache__organism_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_location_cache
    ADD CONSTRAINT mrk_location_cache__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


--
-- Name: mrk_marker mrk_marker__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_marker
    ADD CONSTRAINT mrk_marker__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mrk_marker mrk_marker__marker_status_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_marker
    ADD CONSTRAINT mrk_marker__marker_status_key_fkey FOREIGN KEY (_marker_status_key) REFERENCES mgd.mrk_status(_marker_status_key) DEFERRABLE;


--
-- Name: mrk_marker mrk_marker__marker_type_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_marker
    ADD CONSTRAINT mrk_marker__marker_type_key_fkey FOREIGN KEY (_marker_type_key) REFERENCES mgd.mrk_types(_marker_type_key) DEFERRABLE;


--
-- Name: mrk_marker mrk_marker__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_marker
    ADD CONSTRAINT mrk_marker__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mrk_marker mrk_marker__organism_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_marker
    ADD CONSTRAINT mrk_marker__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


--
-- Name: mrk_mcv_cache mrk_mcv_cache__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_mcv_cache
    ADD CONSTRAINT mrk_mcv_cache__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mrk_mcv_cache mrk_mcv_cache__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_mcv_cache
    ADD CONSTRAINT mrk_mcv_cache__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mrk_mcv_cache mrk_mcv_cache__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_mcv_cache
    ADD CONSTRAINT mrk_mcv_cache__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mrk_mcv_count_cache mrk_mcv_count_cache__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_mcv_count_cache
    ADD CONSTRAINT mrk_mcv_count_cache__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mrk_mcv_count_cache mrk_mcv_count_cache__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_mcv_count_cache
    ADD CONSTRAINT mrk_mcv_count_cache__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mrk_notes mrk_notes__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_notes
    ADD CONSTRAINT mrk_notes__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mrk_reference mrk_reference__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_reference
    ADD CONSTRAINT mrk_reference__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mrk_reference mrk_reference__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_reference
    ADD CONSTRAINT mrk_reference__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mrk_strainmarker mrk_strainmarker__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_strainmarker
    ADD CONSTRAINT mrk_strainmarker__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mrk_strainmarker mrk_strainmarker__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_strainmarker
    ADD CONSTRAINT mrk_strainmarker__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: mrk_strainmarker mrk_strainmarker__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_strainmarker
    ADD CONSTRAINT mrk_strainmarker__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: mrk_strainmarker mrk_strainmarker__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_strainmarker
    ADD CONSTRAINT mrk_strainmarker__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


--
-- Name: mrk_strainmarker mrk_strainmarker__strain_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.mrk_strainmarker
    ADD CONSTRAINT mrk_strainmarker__strain_key_fkey FOREIGN KEY (_strain_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


--
-- Name: prb_alias prb_alias__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_alias
    ADD CONSTRAINT prb_alias__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: prb_alias prb_alias__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_alias
    ADD CONSTRAINT prb_alias__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: prb_alias prb_alias__reference_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_alias
    ADD CONSTRAINT prb_alias__reference_key_fkey FOREIGN KEY (_reference_key) REFERENCES mgd.prb_reference(_reference_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: prb_allele prb_allele__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_allele
    ADD CONSTRAINT prb_allele__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: prb_allele prb_allele__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_allele
    ADD CONSTRAINT prb_allele__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: prb_allele prb_allele__rflv_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_allele
    ADD CONSTRAINT prb_allele__rflv_key_fkey FOREIGN KEY (_rflv_key) REFERENCES mgd.prb_rflv(_rflv_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: prb_allele_strain prb_allele_strain__allele_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_allele_strain
    ADD CONSTRAINT prb_allele_strain__allele_key_fkey FOREIGN KEY (_allele_key) REFERENCES mgd.prb_allele(_allele_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: prb_allele_strain prb_allele_strain__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_allele_strain
    ADD CONSTRAINT prb_allele_strain__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: prb_allele_strain prb_allele_strain__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_allele_strain
    ADD CONSTRAINT prb_allele_strain__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: prb_allele_strain prb_allele_strain__strain_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_allele_strain
    ADD CONSTRAINT prb_allele_strain__strain_key_fkey FOREIGN KEY (_strain_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


--
-- Name: prb_marker prb_marker__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_marker
    ADD CONSTRAINT prb_marker__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: prb_marker prb_marker__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_marker
    ADD CONSTRAINT prb_marker__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- Name: prb_marker prb_marker__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_marker
    ADD CONSTRAINT prb_marker__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: prb_marker prb_marker__probe_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_marker
    ADD CONSTRAINT prb_marker__probe_key_fkey FOREIGN KEY (_probe_key) REFERENCES mgd.prb_probe(_probe_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: prb_marker prb_marker__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_marker
    ADD CONSTRAINT prb_marker__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


--
-- Name: prb_notes prb_notes__probe_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_notes
    ADD CONSTRAINT prb_notes__probe_key_fkey FOREIGN KEY (_probe_key) REFERENCES mgd.prb_probe(_probe_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: prb_probe prb_probe__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_probe
    ADD CONSTRAINT prb_probe__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: prb_probe prb_probe__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_probe
    ADD CONSTRAINT prb_probe__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: prb_probe prb_probe__segmenttype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_probe
    ADD CONSTRAINT prb_probe__segmenttype_key_fkey FOREIGN KEY (_segmenttype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: prb_probe prb_probe__source_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_probe
    ADD CONSTRAINT prb_probe__source_key_fkey FOREIGN KEY (_source_key) REFERENCES mgd.prb_source(_source_key) DEFERRABLE;


--
-- Name: prb_probe prb_probe__vector_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_probe
    ADD CONSTRAINT prb_probe__vector_key_fkey FOREIGN KEY (_vector_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: prb_probe prb_probe_ampprimer_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_probe
    ADD CONSTRAINT prb_probe_ampprimer_fkey FOREIGN KEY (ampprimer) REFERENCES mgd.prb_probe(_probe_key) DEFERRABLE;


--
-- Name: prb_probe prb_probe_derivedfrom_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_probe
    ADD CONSTRAINT prb_probe_derivedfrom_fkey FOREIGN KEY (derivedfrom) REFERENCES mgd.prb_probe(_probe_key) DEFERRABLE;


--
-- Name: prb_ref_notes prb_ref_notes__reference_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_ref_notes
    ADD CONSTRAINT prb_ref_notes__reference_key_fkey FOREIGN KEY (_reference_key) REFERENCES mgd.prb_reference(_reference_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: prb_reference prb_reference__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_reference
    ADD CONSTRAINT prb_reference__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: prb_reference prb_reference__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_reference
    ADD CONSTRAINT prb_reference__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: prb_reference prb_reference__probe_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_reference
    ADD CONSTRAINT prb_reference__probe_key_fkey FOREIGN KEY (_probe_key) REFERENCES mgd.prb_probe(_probe_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: prb_reference prb_reference__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_reference
    ADD CONSTRAINT prb_reference__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


--
-- Name: prb_rflv prb_rflv__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_rflv
    ADD CONSTRAINT prb_rflv__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: prb_rflv prb_rflv__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_rflv
    ADD CONSTRAINT prb_rflv__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- Name: prb_rflv prb_rflv__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_rflv
    ADD CONSTRAINT prb_rflv__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: prb_rflv prb_rflv__reference_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_rflv
    ADD CONSTRAINT prb_rflv__reference_key_fkey FOREIGN KEY (_reference_key) REFERENCES mgd.prb_reference(_reference_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: prb_source prb_source__cellline_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_source
    ADD CONSTRAINT prb_source__cellline_key_fkey FOREIGN KEY (_cellline_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: prb_source prb_source__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_source
    ADD CONSTRAINT prb_source__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: prb_source prb_source__gender_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_source
    ADD CONSTRAINT prb_source__gender_key_fkey FOREIGN KEY (_gender_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: prb_source prb_source__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_source
    ADD CONSTRAINT prb_source__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: prb_source prb_source__organism_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_source
    ADD CONSTRAINT prb_source__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


--
-- Name: prb_source prb_source__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_source
    ADD CONSTRAINT prb_source__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


--
-- Name: prb_source prb_source__segmenttype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_source
    ADD CONSTRAINT prb_source__segmenttype_key_fkey FOREIGN KEY (_segmenttype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: prb_source prb_source__strain_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_source
    ADD CONSTRAINT prb_source__strain_key_fkey FOREIGN KEY (_strain_key) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


--
-- Name: prb_source prb_source__tissue_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_source
    ADD CONSTRAINT prb_source__tissue_key_fkey FOREIGN KEY (_tissue_key) REFERENCES mgd.prb_tissue(_tissue_key) DEFERRABLE;


--
-- Name: prb_source prb_source__vector_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_source
    ADD CONSTRAINT prb_source__vector_key_fkey FOREIGN KEY (_vector_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: prb_strain prb_strain__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_strain
    ADD CONSTRAINT prb_strain__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: prb_strain prb_strain__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_strain
    ADD CONSTRAINT prb_strain__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: prb_strain prb_strain__species_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_strain
    ADD CONSTRAINT prb_strain__species_key_fkey FOREIGN KEY (_species_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: prb_strain prb_strain__straintype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_strain
    ADD CONSTRAINT prb_strain__straintype_key_fkey FOREIGN KEY (_straintype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: prb_strain_genotype prb_strain_genotype__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_strain_genotype
    ADD CONSTRAINT prb_strain_genotype__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: prb_strain_genotype prb_strain_genotype__genotype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_strain_genotype
    ADD CONSTRAINT prb_strain_genotype__genotype_key_fkey FOREIGN KEY (_genotype_key) REFERENCES mgd.gxd_genotype(_genotype_key) DEFERRABLE;


--
-- Name: prb_strain_genotype prb_strain_genotype__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_strain_genotype
    ADD CONSTRAINT prb_strain_genotype__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: prb_strain_genotype prb_strain_genotype__qualifier_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_strain_genotype
    ADD CONSTRAINT prb_strain_genotype__qualifier_key_fkey FOREIGN KEY (_qualifier_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: prb_strain_genotype prb_strain_genotype__strain_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_strain_genotype
    ADD CONSTRAINT prb_strain_genotype__strain_key_fkey FOREIGN KEY (_strain_key) REFERENCES mgd.prb_strain(_strain_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: prb_strain_marker prb_strain_marker__allele_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_strain_marker
    ADD CONSTRAINT prb_strain_marker__allele_key_fkey FOREIGN KEY (_allele_key) REFERENCES mgd.all_allele(_allele_key) DEFERRABLE;


--
-- Name: prb_strain_marker prb_strain_marker__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_strain_marker
    ADD CONSTRAINT prb_strain_marker__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: prb_strain_marker prb_strain_marker__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_strain_marker
    ADD CONSTRAINT prb_strain_marker__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- Name: prb_strain_marker prb_strain_marker__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_strain_marker
    ADD CONSTRAINT prb_strain_marker__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: prb_strain_marker prb_strain_marker__qualifier_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_strain_marker
    ADD CONSTRAINT prb_strain_marker__qualifier_key_fkey FOREIGN KEY (_qualifier_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: prb_strain_marker prb_strain_marker__strain_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.prb_strain_marker
    ADD CONSTRAINT prb_strain_marker__strain_key_fkey FOREIGN KEY (_strain_key) REFERENCES mgd.prb_strain(_strain_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: ri_riset ri_riset__strain_key_1_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.ri_riset
    ADD CONSTRAINT ri_riset__strain_key_1_fkey FOREIGN KEY (_strain_key_1) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


--
-- Name: ri_riset ri_riset__strain_key_2_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.ri_riset
    ADD CONSTRAINT ri_riset__strain_key_2_fkey FOREIGN KEY (_strain_key_2) REFERENCES mgd.prb_strain(_strain_key) DEFERRABLE;


--
-- Name: ri_summary ri_summary__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.ri_summary
    ADD CONSTRAINT ri_summary__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- Name: ri_summary ri_summary__riset_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.ri_summary
    ADD CONSTRAINT ri_summary__riset_key_fkey FOREIGN KEY (_riset_key) REFERENCES mgd.ri_riset(_riset_key) DEFERRABLE;


--
-- Name: ri_summary_expt_ref ri_summary_expt_ref__expt_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.ri_summary_expt_ref
    ADD CONSTRAINT ri_summary_expt_ref__expt_key_fkey FOREIGN KEY (_expt_key) REFERENCES mgd.mld_expts(_expt_key) DEFERRABLE;


--
-- Name: ri_summary_expt_ref ri_summary_expt_ref__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.ri_summary_expt_ref
    ADD CONSTRAINT ri_summary_expt_ref__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


--
-- Name: seq_allele_assoc seq_allele_assoc__allele_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_allele_assoc
    ADD CONSTRAINT seq_allele_assoc__allele_key_fkey FOREIGN KEY (_allele_key) REFERENCES mgd.all_allele(_allele_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: seq_allele_assoc seq_allele_assoc__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_allele_assoc
    ADD CONSTRAINT seq_allele_assoc__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: seq_allele_assoc seq_allele_assoc__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_allele_assoc
    ADD CONSTRAINT seq_allele_assoc__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: seq_allele_assoc seq_allele_assoc__qualifier_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_allele_assoc
    ADD CONSTRAINT seq_allele_assoc__qualifier_key_fkey FOREIGN KEY (_qualifier_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: seq_allele_assoc seq_allele_assoc__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_allele_assoc
    ADD CONSTRAINT seq_allele_assoc__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


--
-- Name: seq_allele_assoc seq_allele_assoc__sequence_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_allele_assoc
    ADD CONSTRAINT seq_allele_assoc__sequence_key_fkey FOREIGN KEY (_sequence_key) REFERENCES mgd.seq_sequence(_sequence_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: seq_coord_cache seq_coord_cache__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_coord_cache
    ADD CONSTRAINT seq_coord_cache__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: seq_coord_cache seq_coord_cache__map_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_coord_cache
    ADD CONSTRAINT seq_coord_cache__map_key_fkey FOREIGN KEY (_map_key) REFERENCES mgd.map_coordinate(_map_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: seq_coord_cache seq_coord_cache__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_coord_cache
    ADD CONSTRAINT seq_coord_cache__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: seq_coord_cache seq_coord_cache__sequence_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_coord_cache
    ADD CONSTRAINT seq_coord_cache__sequence_key_fkey FOREIGN KEY (_sequence_key) REFERENCES mgd.seq_sequence(_sequence_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: seq_genemodel seq_genemodel__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_genemodel
    ADD CONSTRAINT seq_genemodel__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: seq_genemodel seq_genemodel__gmmarker_type_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_genemodel
    ADD CONSTRAINT seq_genemodel__gmmarker_type_key_fkey FOREIGN KEY (_gmmarker_type_key) REFERENCES mgd.mrk_types(_marker_type_key) DEFERRABLE;


--
-- Name: seq_genemodel seq_genemodel__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_genemodel
    ADD CONSTRAINT seq_genemodel__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: seq_genemodel seq_genemodel__sequence_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_genemodel
    ADD CONSTRAINT seq_genemodel__sequence_key_fkey FOREIGN KEY (_sequence_key) REFERENCES mgd.seq_sequence(_sequence_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: seq_genetrap seq_genetrap__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_genetrap
    ADD CONSTRAINT seq_genetrap__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: seq_genetrap seq_genetrap__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_genetrap
    ADD CONSTRAINT seq_genetrap__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: seq_genetrap seq_genetrap__reversecomp_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_genetrap
    ADD CONSTRAINT seq_genetrap__reversecomp_key_fkey FOREIGN KEY (_reversecomp_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: seq_genetrap seq_genetrap__sequence_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_genetrap
    ADD CONSTRAINT seq_genetrap__sequence_key_fkey FOREIGN KEY (_sequence_key) REFERENCES mgd.seq_sequence(_sequence_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: seq_genetrap seq_genetrap__tagmethod_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_genetrap
    ADD CONSTRAINT seq_genetrap__tagmethod_key_fkey FOREIGN KEY (_tagmethod_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: seq_genetrap seq_genetrap__vectorend_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_genetrap
    ADD CONSTRAINT seq_genetrap__vectorend_key_fkey FOREIGN KEY (_vectorend_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: seq_marker_cache seq_marker_cache__biotypeconflict_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_marker_cache
    ADD CONSTRAINT seq_marker_cache__biotypeconflict_key_fkey FOREIGN KEY (_biotypeconflict_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: seq_marker_cache seq_marker_cache__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_marker_cache
    ADD CONSTRAINT seq_marker_cache__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: seq_marker_cache seq_marker_cache__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_marker_cache
    ADD CONSTRAINT seq_marker_cache__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: seq_marker_cache seq_marker_cache__marker_type_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_marker_cache
    ADD CONSTRAINT seq_marker_cache__marker_type_key_fkey FOREIGN KEY (_marker_type_key) REFERENCES mgd.mrk_types(_marker_type_key) DEFERRABLE;


--
-- Name: seq_marker_cache seq_marker_cache__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_marker_cache
    ADD CONSTRAINT seq_marker_cache__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: seq_marker_cache seq_marker_cache__organism_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_marker_cache
    ADD CONSTRAINT seq_marker_cache__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


--
-- Name: seq_marker_cache seq_marker_cache__qualifier_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_marker_cache
    ADD CONSTRAINT seq_marker_cache__qualifier_key_fkey FOREIGN KEY (_qualifier_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: seq_marker_cache seq_marker_cache__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_marker_cache
    ADD CONSTRAINT seq_marker_cache__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: seq_marker_cache seq_marker_cache__sequence_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_marker_cache
    ADD CONSTRAINT seq_marker_cache__sequence_key_fkey FOREIGN KEY (_sequence_key) REFERENCES mgd.seq_sequence(_sequence_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: seq_marker_cache seq_marker_cache__sequenceprovider_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_marker_cache
    ADD CONSTRAINT seq_marker_cache__sequenceprovider_key_fkey FOREIGN KEY (_sequenceprovider_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: seq_marker_cache seq_marker_cache__sequencetype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_marker_cache
    ADD CONSTRAINT seq_marker_cache__sequencetype_key_fkey FOREIGN KEY (_sequencetype_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: seq_probe_cache seq_probe_cache__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_probe_cache
    ADD CONSTRAINT seq_probe_cache__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: seq_probe_cache seq_probe_cache__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_probe_cache
    ADD CONSTRAINT seq_probe_cache__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: seq_probe_cache seq_probe_cache__probe_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_probe_cache
    ADD CONSTRAINT seq_probe_cache__probe_key_fkey FOREIGN KEY (_probe_key) REFERENCES mgd.prb_probe(_probe_key) DEFERRABLE;


--
-- Name: seq_probe_cache seq_probe_cache__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_probe_cache
    ADD CONSTRAINT seq_probe_cache__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: seq_probe_cache seq_probe_cache__sequence_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_probe_cache
    ADD CONSTRAINT seq_probe_cache__sequence_key_fkey FOREIGN KEY (_sequence_key) REFERENCES mgd.seq_sequence(_sequence_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: seq_sequence seq_sequence__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_sequence
    ADD CONSTRAINT seq_sequence__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: seq_sequence seq_sequence__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_sequence
    ADD CONSTRAINT seq_sequence__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: seq_sequence seq_sequence__organism_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_sequence
    ADD CONSTRAINT seq_sequence__organism_key_fkey FOREIGN KEY (_organism_key) REFERENCES mgd.mgi_organism(_organism_key) DEFERRABLE;


--
-- Name: seq_sequence seq_sequence__sequenceprovider_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_sequence
    ADD CONSTRAINT seq_sequence__sequenceprovider_key_fkey FOREIGN KEY (_sequenceprovider_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: seq_sequence seq_sequence__sequencequality_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_sequence
    ADD CONSTRAINT seq_sequence__sequencequality_key_fkey FOREIGN KEY (_sequencequality_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: seq_sequence seq_sequence__sequencestatus_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_sequence
    ADD CONSTRAINT seq_sequence__sequencestatus_key_fkey FOREIGN KEY (_sequencestatus_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: seq_sequence seq_sequence__sequencetype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_sequence
    ADD CONSTRAINT seq_sequence__sequencetype_key_fkey FOREIGN KEY (_sequencetype_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: seq_sequence_assoc seq_sequence_assoc__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_sequence_assoc
    ADD CONSTRAINT seq_sequence_assoc__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: seq_sequence_assoc seq_sequence_assoc__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_sequence_assoc
    ADD CONSTRAINT seq_sequence_assoc__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: seq_sequence_assoc seq_sequence_assoc__qualifier_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_sequence_assoc
    ADD CONSTRAINT seq_sequence_assoc__qualifier_key_fkey FOREIGN KEY (_qualifier_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: seq_sequence_assoc seq_sequence_assoc__sequence_key_1_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_sequence_assoc
    ADD CONSTRAINT seq_sequence_assoc__sequence_key_1_fkey FOREIGN KEY (_sequence_key_1) REFERENCES mgd.seq_sequence(_sequence_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: seq_sequence_assoc seq_sequence_assoc__sequence_key_2_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_sequence_assoc
    ADD CONSTRAINT seq_sequence_assoc__sequence_key_2_fkey FOREIGN KEY (_sequence_key_2) REFERENCES mgd.seq_sequence(_sequence_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: seq_sequence_raw seq_sequence_raw__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_sequence_raw
    ADD CONSTRAINT seq_sequence_raw__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: seq_sequence_raw seq_sequence_raw__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_sequence_raw
    ADD CONSTRAINT seq_sequence_raw__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: seq_sequence_raw seq_sequence_raw__sequence_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_sequence_raw
    ADD CONSTRAINT seq_sequence_raw__sequence_key_fkey FOREIGN KEY (_sequence_key) REFERENCES mgd.seq_sequence(_sequence_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: seq_source_assoc seq_source_assoc__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_source_assoc
    ADD CONSTRAINT seq_source_assoc__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: seq_source_assoc seq_source_assoc__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_source_assoc
    ADD CONSTRAINT seq_source_assoc__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: seq_source_assoc seq_source_assoc__sequence_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_source_assoc
    ADD CONSTRAINT seq_source_assoc__sequence_key_fkey FOREIGN KEY (_sequence_key) REFERENCES mgd.seq_sequence(_sequence_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: seq_source_assoc seq_source_assoc__source_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.seq_source_assoc
    ADD CONSTRAINT seq_source_assoc__source_key_fkey FOREIGN KEY (_source_key) REFERENCES mgd.prb_source(_source_key) DEFERRABLE;


--
-- Name: voc_allele_cache voc_allele_cache__allele_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_allele_cache
    ADD CONSTRAINT voc_allele_cache__allele_key_fkey FOREIGN KEY (_allele_key) REFERENCES mgd.all_allele(_allele_key) DEFERRABLE;


--
-- Name: voc_allele_cache voc_allele_cache__term_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_allele_cache
    ADD CONSTRAINT voc_allele_cache__term_key_fkey FOREIGN KEY (_term_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: voc_annot voc_annot__annottype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_annot
    ADD CONSTRAINT voc_annot__annottype_key_fkey FOREIGN KEY (_annottype_key) REFERENCES mgd.voc_annottype(_annottype_key) DEFERRABLE;


--
-- Name: voc_annot voc_annot__qualifier_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_annot
    ADD CONSTRAINT voc_annot__qualifier_key_fkey FOREIGN KEY (_qualifier_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: voc_annot voc_annot__term_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_annot
    ADD CONSTRAINT voc_annot__term_key_fkey FOREIGN KEY (_term_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: voc_annot_count_cache voc_annot_count_cache__term_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_annot_count_cache
    ADD CONSTRAINT voc_annot_count_cache__term_key_fkey FOREIGN KEY (_term_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: voc_annotheader voc_annotheader__annottype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_annotheader
    ADD CONSTRAINT voc_annotheader__annottype_key_fkey FOREIGN KEY (_annottype_key) REFERENCES mgd.voc_annottype(_annottype_key) DEFERRABLE;


--
-- Name: voc_annotheader voc_annotheader__approvedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_annotheader
    ADD CONSTRAINT voc_annotheader__approvedby_key_fkey FOREIGN KEY (_approvedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: voc_annotheader voc_annotheader__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_annotheader
    ADD CONSTRAINT voc_annotheader__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: voc_annotheader voc_annotheader__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_annotheader
    ADD CONSTRAINT voc_annotheader__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: voc_annotheader voc_annotheader__term_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_annotheader
    ADD CONSTRAINT voc_annotheader__term_key_fkey FOREIGN KEY (_term_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: voc_annottype voc_annottype__evidencevocab_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_annottype
    ADD CONSTRAINT voc_annottype__evidencevocab_key_fkey FOREIGN KEY (_evidencevocab_key) REFERENCES mgd.voc_vocab(_vocab_key) DEFERRABLE;


--
-- Name: voc_annottype voc_annottype__mgitype_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_annottype
    ADD CONSTRAINT voc_annottype__mgitype_key_fkey FOREIGN KEY (_mgitype_key) REFERENCES mgd.acc_mgitype(_mgitype_key) DEFERRABLE;


--
-- Name: voc_annottype voc_annottype__qualifiervocab_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_annottype
    ADD CONSTRAINT voc_annottype__qualifiervocab_key_fkey FOREIGN KEY (_qualifiervocab_key) REFERENCES mgd.voc_vocab(_vocab_key) DEFERRABLE;


--
-- Name: voc_annottype voc_annottype__vocab_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_annottype
    ADD CONSTRAINT voc_annottype__vocab_key_fkey FOREIGN KEY (_vocab_key) REFERENCES mgd.voc_vocab(_vocab_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: voc_evidence voc_evidence__annot_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_evidence
    ADD CONSTRAINT voc_evidence__annot_key_fkey FOREIGN KEY (_annot_key) REFERENCES mgd.voc_annot(_annot_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: voc_evidence voc_evidence__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_evidence
    ADD CONSTRAINT voc_evidence__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: voc_evidence voc_evidence__evidenceterm_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_evidence
    ADD CONSTRAINT voc_evidence__evidenceterm_key_fkey FOREIGN KEY (_evidenceterm_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: voc_evidence voc_evidence__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_evidence
    ADD CONSTRAINT voc_evidence__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: voc_evidence voc_evidence__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_evidence
    ADD CONSTRAINT voc_evidence__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


--
-- Name: voc_evidence_property voc_evidence_property__annotevidence_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_evidence_property
    ADD CONSTRAINT voc_evidence_property__annotevidence_key_fkey FOREIGN KEY (_annotevidence_key) REFERENCES mgd.voc_evidence(_annotevidence_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: voc_evidence_property voc_evidence_property__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_evidence_property
    ADD CONSTRAINT voc_evidence_property__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: voc_evidence_property voc_evidence_property__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_evidence_property
    ADD CONSTRAINT voc_evidence_property__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: voc_evidence_property voc_evidence_property__propertyterm_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_evidence_property
    ADD CONSTRAINT voc_evidence_property__propertyterm_key_fkey FOREIGN KEY (_propertyterm_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: voc_marker_cache voc_marker_cache__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_marker_cache
    ADD CONSTRAINT voc_marker_cache__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: voc_marker_cache voc_marker_cache__term_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_marker_cache
    ADD CONSTRAINT voc_marker_cache__term_key_fkey FOREIGN KEY (_term_key) REFERENCES mgd.voc_term(_term_key) DEFERRABLE;


--
-- Name: voc_term voc_term__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_term
    ADD CONSTRAINT voc_term__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: voc_term voc_term__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_term
    ADD CONSTRAINT voc_term__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: voc_term voc_term__vocab_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_term
    ADD CONSTRAINT voc_term__vocab_key_fkey FOREIGN KEY (_vocab_key) REFERENCES mgd.voc_vocab(_vocab_key) DEFERRABLE;


--
-- Name: voc_term_emapa voc_term_emapa__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_term_emapa
    ADD CONSTRAINT voc_term_emapa__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: voc_term_emapa voc_term_emapa__defaultparent_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_term_emapa
    ADD CONSTRAINT voc_term_emapa__defaultparent_key_fkey FOREIGN KEY (_defaultparent_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: voc_term_emapa voc_term_emapa__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_term_emapa
    ADD CONSTRAINT voc_term_emapa__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: voc_term_emapa voc_term_emapa__term_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_term_emapa
    ADD CONSTRAINT voc_term_emapa__term_key_fkey FOREIGN KEY (_term_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: voc_term_emapa voc_term_emapa_endstage_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_term_emapa
    ADD CONSTRAINT voc_term_emapa_endstage_fkey FOREIGN KEY (endstage) REFERENCES mgd.gxd_theilerstage(_stage_key) DEFERRABLE;


--
-- Name: voc_term_emapa voc_term_emapa_startstage_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_term_emapa
    ADD CONSTRAINT voc_term_emapa_startstage_fkey FOREIGN KEY (startstage) REFERENCES mgd.gxd_theilerstage(_stage_key) DEFERRABLE;


--
-- Name: voc_term_emaps voc_term_emaps__createdby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_term_emaps
    ADD CONSTRAINT voc_term_emaps__createdby_key_fkey FOREIGN KEY (_createdby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: voc_term_emaps voc_term_emaps__defaultparent_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_term_emaps
    ADD CONSTRAINT voc_term_emaps__defaultparent_key_fkey FOREIGN KEY (_defaultparent_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: voc_term_emaps voc_term_emaps__emapa_term_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_term_emaps
    ADD CONSTRAINT voc_term_emaps__emapa_term_key_fkey FOREIGN KEY (_emapa_term_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: voc_term_emaps voc_term_emaps__modifiedby_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_term_emaps
    ADD CONSTRAINT voc_term_emaps__modifiedby_key_fkey FOREIGN KEY (_modifiedby_key) REFERENCES mgd.mgi_user(_user_key) DEFERRABLE;


--
-- Name: voc_term_emaps voc_term_emaps__stage_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_term_emaps
    ADD CONSTRAINT voc_term_emaps__stage_key_fkey FOREIGN KEY (_stage_key) REFERENCES mgd.gxd_theilerstage(_stage_key) DEFERRABLE;


--
-- Name: voc_term_emaps voc_term_emaps__term_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_term_emaps
    ADD CONSTRAINT voc_term_emaps__term_key_fkey FOREIGN KEY (_term_key) REFERENCES mgd.voc_term(_term_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: voc_vocab voc_vocab__logicaldb_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_vocab
    ADD CONSTRAINT voc_vocab__logicaldb_key_fkey FOREIGN KEY (_logicaldb_key) REFERENCES mgd.acc_logicaldb(_logicaldb_key) DEFERRABLE;


--
-- Name: voc_vocab voc_vocab__refs_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_vocab
    ADD CONSTRAINT voc_vocab__refs_key_fkey FOREIGN KEY (_refs_key) REFERENCES mgd.bib_refs(_refs_key) DEFERRABLE;


--
-- Name: voc_vocabdag voc_vocabdag__dag_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_vocabdag
    ADD CONSTRAINT voc_vocabdag__dag_key_fkey FOREIGN KEY (_dag_key) REFERENCES mgd.dag_dag(_dag_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: voc_vocabdag voc_vocabdag__vocab_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.voc_vocabdag
    ADD CONSTRAINT voc_vocabdag__vocab_key_fkey FOREIGN KEY (_vocab_key) REFERENCES mgd.voc_vocab(_vocab_key) ON DELETE CASCADE DEFERRABLE;


--
-- Name: wks_rosetta wks_rosetta__marker_key_fkey; Type: FK CONSTRAINT; Schema: mgd; Owner: -
--

ALTER TABLE ONLY mgd.wks_rosetta
    ADD CONSTRAINT wks_rosetta__marker_key_fkey FOREIGN KEY (_marker_key) REFERENCES mgd.mrk_marker(_marker_key) DEFERRABLE;


--
-- PostgreSQL database dump complete
--


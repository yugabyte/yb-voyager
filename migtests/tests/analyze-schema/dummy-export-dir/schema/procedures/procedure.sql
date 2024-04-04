--drop temporary table clause
CREATE OR REPLACE PROCEDURE foo (p_id integer) AS $body$
BEGIN
    drop temporary table if exists temp;

    create temporary table temp(id int, name text);

    insert into temp(id,name) select id,p_name from bar where p_id=id;

    select name from temp;

end;
$body$
LANGUAGE PLPGSQL
SECURITY DEFINER
;

--JSON_ARRAYAGG
CREATE OR REPLACE PROCEDURE foo1 (p_id integer) AS $body$
BEGIN

    create temporary table temp(id int, agg bigint);

    insert into temp(id,agg) select x , JSON_ARRAYAGG(trunc(b, 2) order by t desc) as agg FROM test1

    select agg from temp;

end;
$body$
LANGUAGE PLPGSQL
SECURITY DEFINER
;


CREATE OR REPLACE PROCEDURE sp_createnachabatch (IN innerXrefid varchar(36)) AS $body$
DECLARE
CUR_GET_XREFCOUNT CURSOR FOR(SELECT count(*)
        FROM gateway.tmp_MDamts
        WHERE t_Dxrefid = innerXrefid);
CUR_GET_DETAILS CURSOR FOR(SELECT t_ppid , t_FullLegalName , t_TradingName,t_OperationCountry ,
        t_FirstName,t_LastName	,t_Country,t_State,t_BirthCity, t_Gender,
        t_id, t_DinquiryType, t_DFirstName, t_DLastName, t_Drtn, t_DAccountNumber,
        t_Dssn, t_DAddress1, t_DCity,t_DState, t_DPhone, t_DIDno, t_DGender,
        t_DrequestStatus , t_DxrefId, t_DtransactionId , MDamt
        FROM gateway.tmp_MDamts
        WHERE t_Dxrefid = innerXrefid);
BEGIN
	
    FETCH CUR_GET_XREFCOUNT INTO intTotCount;
   OPEN CUR_GET_DETAILS;
    var_Final_JString := '';
    var_Total_Remit := 0;
	var_SepComma := ',';
    push_createpayment:LOOP
		If (int_clnt_id > intTotCount) then
			var_SepComma := '';
			EXIT push_createpayment;
        END IF;
		FETCH CUR_GET_DETAILS INTO var_ppid,var_FullLegalName,var_TradingName,var_OperationCountry,
		var_FirstName,var_LastName,var_Country, var_State, var_BirthCity, var_tGender,var_id,
		var_DinquiryType,var_DFirstName,var_DLastName,
        var_Drtn,var_DAccountNumber,var_Dssn,var_DAddress1,
		var_DCity,var_DState,var_DPhone, var_DIDno, var_DGender, var_DrequestStatus,var_DxrefId,var_DtransactionId,var_MDamt;
        var_Total_Remit := var_Total_Remit + var_MDamt;
		If (int_clnt_id = 1) then
			var_BNYM := 'BNY Mellon';
			var_clnt_id := concat(concat(lpad(extract(day from date(day))::integer,2,0),lpad(extract(month from date(sysdate()))::integer,2,0),extract(year from date(sysdate()))),lpad(int_clnt_id,7,0));
			var_RemitDate := concat(lpad(extract(day from date(day))::integer,2,0),'-',upper(substring(to_char((sysdate())::date, 'FMMonth'),1,3)),'-',extract(year from date(sysdate())));
			jsonCPayment:=concat(' ',array_to_string(ARRAY(SELECT chr(unnest(34))),''),'http://soa01-uat-connectpay.eastus.cloudapp.azure.com:37005/gts/1.0/Createpayment',array_to_string(ARRAY(SELECT chr(unnest(34))),''),' \\ ');
		End if;
		
    END LOOP push_createpayment;
    json1:=concat('{',array_to_string(ARRAY(SELECT chr(unnest(13))),''),array_to_string(ARRAY(SELECT chr(unnest(34))),''),'header',array_to_string(ARRAY(SELECT chr(unnest(34))),''),': {',array_to_string(ARRAY(SELECT chr(unnest(13))),''),array_to_string(ARRAY(SELECT chr(unnest(34))),''),'SendingOrgID',array_to_string(ARRAY(SELECT chr(unnest(34))),''),':',array_to_string(ARRAY(SELECT chr(unnest(34))),''),var_ppid,array_to_string(ARRAY(SELECT chr(unnest(34))),''),',',array_to_string(ARRAY(SELECT chr(unnest(13))),''),array_to_string(ARRAY(SELECT chr(unnest(34))),''),'BatchNumber',array_to_string(ARRAY(SELECT chr(unnest(34))),''),':',array_to_string(ARRAY(SELECT chr(unnest(34))),''), var_id,array_to_string(ARRAY(SELECT chr(unnest(34))),''),',',array_to_string(ARRAY(SELECT chr(unnest(13))),''),array_to_string(ARRAY(SELECT chr(unnest(34))),''),'RemittanceCount',array_to_string(ARRAY(SELECT chr(unnest(34))),''),': ',array_to_string(ARRAY(SELECT chr(unnest(34))),''),int_clnt_id - 1,array_to_string(ARRAY(SELECT chr(unnest(34))),''),',',array_to_string(ARRAY(SELECT chr(unnest(13))),''),array_to_string(ARRAY(SELECT chr(unnest(34))),''),'ControlSum',array_to_string(ARRAY(SELECT chr(unnest(34))),''),':',array_to_string(ARRAY(SELECT chr(unnest(34))),''),var_Total_Remit,array_to_string(ARRAY(SELECT chr(unnest(34))),''),'},');
	var_Final_Jstring := CONCAT(array_to_string(ARRAY(SELECT chr(unnest(34))),''),json1,var_Final_Jstring,'}',array_to_string(ARRAY(SELECT chr(unnest(34))),''));
	
	INSERT INTO gateway.tmp_curl_commands(t_cpjsonrequest) values (var_Final_JString);
    COMMIT;
	CLOSE CUR_GET_XREFCOUNT;
    CLOSE CUR_GET_DETAILS;
END;
$body$
LANGUAGE PLPGSQL
SECURITY DEFINER
;


CREATE OR REPLACE PROCEDURE test () AS $body$
DECLARE 
    cur CURSOR FOR SELECT column_name FROM table_name;
    row RECORD;
BEGIN
    OPEN cur;
    FETCH PRIOR FROM cur INTO row;
    CLOSE cur;
END;
$body$
LANGUAGE PLPGSQL
SECURITY DEFINER
;


CREATE OR REPLACE PROCEDURE test1 () AS $body$
DECLARE 
    cur CURSOR FOR SELECT column_name FROM table_name;
    row RECORD;
BEGIN
    OPEN cur;
    FETCH cur INTO row;
    CLOSE cur;
END;
$body$
LANGUAGE PLPGSQL
SECURITY DEFINER
;

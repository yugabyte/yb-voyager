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


CREATE TABLE humanresources.department (
    departmentid integer NOT NULL,
    name public."Name" NOT NULL,
    groupname public."Name" NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE humanresources.employee (
    businessentityid integer NOT NULL,
    nationalidnumber character varying(15) NOT NULL,
    loginid character varying(256) NOT NULL,
    jobtitle character varying(50) NOT NULL,
    birthdate date NOT NULL,
    maritalstatus character(1) NOT NULL,
    gender character(1) NOT NULL,
    hiredate date NOT NULL,
    salariedflag public."Flag" DEFAULT true NOT NULL,
    vacationhours smallint DEFAULT 0 NOT NULL,
    sickleavehours smallint DEFAULT 0 NOT NULL,
    currentflag public."Flag" DEFAULT true NOT NULL,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    organizationnode character varying DEFAULT '/'::character varying,
    CONSTRAINT "CK_Employee_BirthDate" CHECK (((birthdate >= '1930-01-01'::date) AND (birthdate <= (now() - '18 years'::interval)))),
    CONSTRAINT "CK_Employee_Gender" CHECK ((upper((gender)::text) = ANY (ARRAY['M'::text, 'F'::text]))),
    CONSTRAINT "CK_Employee_HireDate" CHECK (((hiredate >= '1996-07-01'::date) AND (hiredate <= (now() + '1 day'::interval)))),
    CONSTRAINT "CK_Employee_MaritalStatus" CHECK ((upper((maritalstatus)::text) = ANY (ARRAY['M'::text, 'S'::text]))),
    CONSTRAINT "CK_Employee_SickLeaveHours" CHECK (((sickleavehours >= 0) AND (sickleavehours <= 120))),
    CONSTRAINT "CK_Employee_VacationHours" CHECK (((vacationhours >= '-40'::integer) AND (vacationhours <= 240)))
);


CREATE TABLE humanresources.employeedepartmenthistory (
    businessentityid integer NOT NULL,
    departmentid smallint NOT NULL,
    shiftid smallint NOT NULL,
    startdate date NOT NULL,
    enddate date,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_EmployeeDepartmentHistory_EndDate" CHECK (((enddate >= startdate) OR (enddate IS NULL)))
);


CREATE TABLE humanresources.employeepayhistory (
    businessentityid integer NOT NULL,
    ratechangedate timestamp without time zone NOT NULL,
    rate numeric NOT NULL,
    payfrequency smallint NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_EmployeePayHistory_PayFrequency" CHECK ((payfrequency = ANY (ARRAY[1, 2]))),
    CONSTRAINT "CK_EmployeePayHistory_Rate" CHECK (((rate >= 6.50) AND (rate <= 200.00)))
);


CREATE TABLE humanresources.jobcandidate (
    jobcandidateid integer NOT NULL,
    businessentityid integer,
    resume xml,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE humanresources.shift (
    shiftid integer NOT NULL,
    name public."Name" NOT NULL,
    starttime time without time zone NOT NULL,
    endtime time without time zone NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE person.address (
    addressid integer NOT NULL,
    addressline1 character varying(60) NOT NULL,
    addressline2 character varying(60),
    city character varying(30) NOT NULL,
    stateprovinceid integer NOT NULL,
    postalcode character varying(15) NOT NULL,
    spatiallocation bytea,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE person.businessentityaddress (
    businessentityid integer NOT NULL,
    addressid integer NOT NULL,
    addresstypeid integer NOT NULL,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE person.countryregion (
    countryregioncode character varying(3) NOT NULL,
    name public."Name" NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE person.emailaddress (
    businessentityid integer NOT NULL,
    emailaddressid integer NOT NULL,
    emailaddress character varying(50),
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE person.person (
    businessentityid integer NOT NULL,
    persontype character(2) NOT NULL,
    namestyle public."NameStyle" DEFAULT false NOT NULL,
    title character varying(8),
    firstname public."Name" NOT NULL,
    middlename public."Name",
    lastname public."Name" NOT NULL,
    suffix character varying(10),
    emailpromotion integer DEFAULT 0 NOT NULL,
    additionalcontactinfo xml,
    demographics xml,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_Person_EmailPromotion" CHECK (((emailpromotion >= 0) AND (emailpromotion <= 2))),
    CONSTRAINT "CK_Person_PersonType" CHECK (((persontype IS NULL) OR (upper((persontype)::text) = ANY (ARRAY['SC'::text, 'VC'::text, 'IN'::text, 'EM'::text, 'SP'::text, 'GC'::text]))))
);


CREATE TABLE person.personphone (
    businessentityid integer NOT NULL,
    phonenumber public."Phone" NOT NULL,
    phonenumbertypeid integer NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE person.phonenumbertype (
    phonenumbertypeid integer NOT NULL,
    name public."Name" NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE person.stateprovince (
    stateprovinceid integer NOT NULL,
    stateprovincecode character(3) NOT NULL,
    countryregioncode character varying(3) NOT NULL,
    isonlystateprovinceflag public."Flag" DEFAULT true NOT NULL,
    name public."Name" NOT NULL,
    territoryid integer NOT NULL,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE person.addresstype (
    addresstypeid integer NOT NULL,
    name public."Name" NOT NULL,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE person.businessentity (
    businessentityid integer NOT NULL,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE person.businessentitycontact (
    businessentityid integer NOT NULL,
    personid integer NOT NULL,
    contacttypeid integer NOT NULL,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE person.contacttype (
    contacttypeid integer NOT NULL,
    name public."Name" NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE person.password (
    businessentityid integer NOT NULL,
    passwordhash character varying(128) NOT NULL,
    passwordsalt character varying(10) NOT NULL,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE production.billofmaterials (
    billofmaterialsid integer NOT NULL,
    productassemblyid integer,
    componentid integer NOT NULL,
    startdate timestamp without time zone DEFAULT now() NOT NULL,
    enddate timestamp without time zone,
    unitmeasurecode character(3) NOT NULL,
    bomlevel smallint NOT NULL,
    perassemblyqty numeric(8,2) DEFAULT 1.00 NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_BillOfMaterials_BOMLevel" CHECK ((((productassemblyid IS NULL) AND (bomlevel = 0) AND (perassemblyqty = 1.00)) OR ((productassemblyid IS NOT NULL) AND (bomlevel >= 1)))),
    CONSTRAINT "CK_BillOfMaterials_EndDate" CHECK (((enddate > startdate) OR (enddate IS NULL))),
    CONSTRAINT "CK_BillOfMaterials_PerAssemblyQty" CHECK ((perassemblyqty >= 1.00)),
    CONSTRAINT "CK_BillOfMaterials_ProductAssemblyID" CHECK ((productassemblyid <> componentid))
);


CREATE TABLE production.culture (
    cultureid character(6) NOT NULL,
    name public."Name" NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE production.document (
    title character varying(50) NOT NULL,
    owner integer NOT NULL,
    folderflag public."Flag" DEFAULT false NOT NULL,
    filename character varying(400) NOT NULL,
    fileextension character varying(8),
    revision character(5) NOT NULL,
    changenumber integer DEFAULT 0 NOT NULL,
    status smallint NOT NULL,
    documentsummary text,
    document bytea,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    documentnode character varying DEFAULT '/'::character varying NOT NULL,
    CONSTRAINT "CK_Document_Status" CHECK (((status >= 1) AND (status <= 3)))
);


CREATE TABLE production.illustration (
    illustrationid integer NOT NULL,
    diagram xml,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE production.location (
    locationid integer NOT NULL,
    name public."Name" NOT NULL,
    costrate numeric DEFAULT 0.00 NOT NULL,
    availability numeric(8,2) DEFAULT 0.00 NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_Location_Availability" CHECK ((availability >= 0.00)),
    CONSTRAINT "CK_Location_CostRate" CHECK ((costrate >= 0.00))
);


CREATE TABLE production.product (
    productid integer NOT NULL,
    name public."Name" NOT NULL,
    productnumber character varying(25) NOT NULL,
    makeflag public."Flag" DEFAULT true NOT NULL,
    finishedgoodsflag public."Flag" DEFAULT true NOT NULL,
    color character varying(15),
    safetystocklevel smallint NOT NULL,
    reorderpoint smallint NOT NULL,
    standardcost numeric NOT NULL,
    listprice numeric NOT NULL,
    size character varying(5),
    sizeunitmeasurecode character(3),
    weightunitmeasurecode character(3),
    weight numeric(8,2),
    daystomanufacture integer NOT NULL,
    productline character(2),
    class character(2),
    style character(2),
    productsubcategoryid integer,
    productmodelid integer,
    sellstartdate timestamp without time zone NOT NULL,
    sellenddate timestamp without time zone,
    discontinueddate timestamp without time zone,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_Product_Class" CHECK (((upper((class)::text) = ANY (ARRAY['L'::text, 'M'::text, 'H'::text])) OR (class IS NULL))),
    CONSTRAINT "CK_Product_DaysToManufacture" CHECK ((daystomanufacture >= 0)),
    CONSTRAINT "CK_Product_ListPrice" CHECK ((listprice >= 0.00)),
    CONSTRAINT "CK_Product_ProductLine" CHECK (((upper((productline)::text) = ANY (ARRAY['S'::text, 'T'::text, 'M'::text, 'R'::text])) OR (productline IS NULL))),
    CONSTRAINT "CK_Product_ReorderPoint" CHECK ((reorderpoint > 0)),
    CONSTRAINT "CK_Product_SafetyStockLevel" CHECK ((safetystocklevel > 0)),
    CONSTRAINT "CK_Product_SellEndDate" CHECK (((sellenddate >= sellstartdate) OR (sellenddate IS NULL))),
    CONSTRAINT "CK_Product_StandardCost" CHECK ((standardcost >= 0.00)),
    CONSTRAINT "CK_Product_Style" CHECK (((upper((style)::text) = ANY (ARRAY['W'::text, 'M'::text, 'U'::text])) OR (style IS NULL))),
    CONSTRAINT "CK_Product_Weight" CHECK ((weight > 0.00))
);


CREATE TABLE production.productcategory (
    productcategoryid integer NOT NULL,
    name public."Name" NOT NULL,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE production.productcosthistory (
    productid integer NOT NULL,
    startdate timestamp without time zone NOT NULL,
    enddate timestamp without time zone,
    standardcost numeric NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_ProductCostHistory_EndDate" CHECK (((enddate >= startdate) OR (enddate IS NULL))),
    CONSTRAINT "CK_ProductCostHistory_StandardCost" CHECK ((standardcost >= 0.00))
);


CREATE TABLE production.productdescription (
    productdescriptionid integer NOT NULL,
    description character varying(400) NOT NULL,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE production.productdocument (
    productid integer NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    documentnode character varying DEFAULT '/'::character varying NOT NULL
);


CREATE TABLE production.productinventory (
    productid integer NOT NULL,
    locationid smallint NOT NULL,
    shelf character varying(10) NOT NULL,
    bin smallint NOT NULL,
    quantity smallint DEFAULT 0 NOT NULL,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_ProductInventory_Bin" CHECK (((bin >= 0) AND (bin <= 100)))
);


CREATE TABLE production.productlistpricehistory (
    productid integer NOT NULL,
    startdate timestamp without time zone NOT NULL,
    enddate timestamp without time zone,
    listprice numeric NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_ProductListPriceHistory_EndDate" CHECK (((enddate >= startdate) OR (enddate IS NULL))),
    CONSTRAINT "CK_ProductListPriceHistory_ListPrice" CHECK ((listprice > 0.00))
);


CREATE TABLE production.productmodel (
    productmodelid integer NOT NULL,
    name public."Name" NOT NULL,
    catalogdescription xml,
    instructions xml,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE production.productmodelillustration (
    productmodelid integer NOT NULL,
    illustrationid integer NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE production.productmodelproductdescriptionculture (
    productmodelid integer NOT NULL,
    productdescriptionid integer NOT NULL,
    cultureid character(6) NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE production.productphoto (
    productphotoid integer NOT NULL,
    thumbnailphoto bytea,
    thumbnailphotofilename character varying(50),
    largephoto bytea,
    largephotofilename character varying(50),
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE production.productproductphoto (
    productid integer NOT NULL,
    productphotoid integer NOT NULL,
    "primary" public."Flag" DEFAULT false NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE production.productreview (
    productreviewid integer NOT NULL,
    productid integer NOT NULL,
    reviewername public."Name" NOT NULL,
    reviewdate timestamp without time zone DEFAULT now() NOT NULL,
    emailaddress character varying(50) NOT NULL,
    rating integer NOT NULL,
    comments character varying(3850),
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_ProductReview_Rating" CHECK (((rating >= 1) AND (rating <= 5)))
);


CREATE TABLE production.productsubcategory (
    productsubcategoryid integer NOT NULL,
    productcategoryid integer NOT NULL,
    name public."Name" NOT NULL,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE production.scrapreason (
    scrapreasonid integer NOT NULL,
    name public."Name" NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE production.transactionhistory (
    transactionid integer NOT NULL,
    productid integer NOT NULL,
    referenceorderid integer NOT NULL,
    referenceorderlineid integer DEFAULT 0 NOT NULL,
    transactiondate timestamp without time zone DEFAULT now() NOT NULL,
    transactiontype character(1) NOT NULL,
    quantity integer NOT NULL,
    actualcost numeric NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_TransactionHistory_TransactionType" CHECK ((upper((transactiontype)::text) = ANY (ARRAY['W'::text, 'S'::text, 'P'::text])))
);


CREATE TABLE production.transactionhistoryarchive (
    transactionid integer NOT NULL,
    productid integer NOT NULL,
    referenceorderid integer NOT NULL,
    referenceorderlineid integer DEFAULT 0 NOT NULL,
    transactiondate timestamp without time zone DEFAULT now() NOT NULL,
    transactiontype character(1) NOT NULL,
    quantity integer NOT NULL,
    actualcost numeric NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_TransactionHistoryArchive_TransactionType" CHECK ((upper((transactiontype)::text) = ANY (ARRAY['W'::text, 'S'::text, 'P'::text])))
);


CREATE TABLE production.unitmeasure (
    unitmeasurecode character(3) NOT NULL,
    name public."Name" NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE production.workorder (
    workorderid integer NOT NULL,
    productid integer NOT NULL,
    orderqty integer NOT NULL,
    scrappedqty smallint NOT NULL,
    startdate timestamp without time zone NOT NULL,
    enddate timestamp without time zone,
    duedate timestamp without time zone NOT NULL,
    scrapreasonid smallint,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_WorkOrder_EndDate" CHECK (((enddate >= startdate) OR (enddate IS NULL))),
    CONSTRAINT "CK_WorkOrder_OrderQty" CHECK ((orderqty > 0)),
    CONSTRAINT "CK_WorkOrder_ScrappedQty" CHECK ((scrappedqty >= 0))
);


CREATE TABLE production.workorderrouting (
    workorderid integer NOT NULL,
    productid integer NOT NULL,
    operationsequence smallint NOT NULL,
    locationid smallint NOT NULL,
    scheduledstartdate timestamp without time zone NOT NULL,
    scheduledenddate timestamp without time zone NOT NULL,
    actualstartdate timestamp without time zone,
    actualenddate timestamp without time zone,
    actualresourcehrs numeric(9,4),
    plannedcost numeric NOT NULL,
    actualcost numeric,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_WorkOrderRouting_ActualCost" CHECK ((actualcost > 0.00)),
    CONSTRAINT "CK_WorkOrderRouting_ActualEndDate" CHECK (((actualenddate >= actualstartdate) OR (actualenddate IS NULL) OR (actualstartdate IS NULL))),
    CONSTRAINT "CK_WorkOrderRouting_ActualResourceHrs" CHECK ((actualresourcehrs >= 0.0000)),
    CONSTRAINT "CK_WorkOrderRouting_PlannedCost" CHECK ((plannedcost > 0.00)),
    CONSTRAINT "CK_WorkOrderRouting_ScheduledEndDate" CHECK ((scheduledenddate >= scheduledstartdate))
);


CREATE TABLE purchasing.purchaseorderdetail (
    purchaseorderid integer NOT NULL,
    purchaseorderdetailid integer NOT NULL,
    duedate timestamp without time zone NOT NULL,
    orderqty smallint NOT NULL,
    productid integer NOT NULL,
    unitprice numeric NOT NULL,
    receivedqty numeric(8,2) NOT NULL,
    rejectedqty numeric(8,2) NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_PurchaseOrderDetail_OrderQty" CHECK ((orderqty > 0)),
    CONSTRAINT "CK_PurchaseOrderDetail_ReceivedQty" CHECK ((receivedqty >= 0.00)),
    CONSTRAINT "CK_PurchaseOrderDetail_RejectedQty" CHECK ((rejectedqty >= 0.00)),
    CONSTRAINT "CK_PurchaseOrderDetail_UnitPrice" CHECK ((unitprice >= 0.00))
);


CREATE TABLE purchasing.purchaseorderheader (
    purchaseorderid integer NOT NULL,
    revisionnumber smallint DEFAULT 0 NOT NULL,
    status smallint DEFAULT 1 NOT NULL,
    employeeid integer NOT NULL,
    vendorid integer NOT NULL,
    shipmethodid integer NOT NULL,
    orderdate timestamp without time zone DEFAULT now() NOT NULL,
    shipdate timestamp without time zone,
    subtotal numeric DEFAULT 0.00 NOT NULL,
    taxamt numeric DEFAULT 0.00 NOT NULL,
    freight numeric DEFAULT 0.00 NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_PurchaseOrderHeader_Freight" CHECK ((freight >= 0.00)),
    CONSTRAINT "CK_PurchaseOrderHeader_ShipDate" CHECK (((shipdate >= orderdate) OR (shipdate IS NULL))),
    CONSTRAINT "CK_PurchaseOrderHeader_Status" CHECK (((status >= 1) AND (status <= 4))),
    CONSTRAINT "CK_PurchaseOrderHeader_SubTotal" CHECK ((subtotal >= 0.00)),
    CONSTRAINT "CK_PurchaseOrderHeader_TaxAmt" CHECK ((taxamt >= 0.00))
);


CREATE TABLE purchasing.productvendor (
    productid integer NOT NULL,
    businessentityid integer NOT NULL,
    averageleadtime integer NOT NULL,
    standardprice numeric NOT NULL,
    lastreceiptcost numeric,
    lastreceiptdate timestamp without time zone,
    minorderqty integer NOT NULL,
    maxorderqty integer NOT NULL,
    onorderqty integer,
    unitmeasurecode character(3) NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_ProductVendor_AverageLeadTime" CHECK ((averageleadtime >= 1)),
    CONSTRAINT "CK_ProductVendor_LastReceiptCost" CHECK ((lastreceiptcost > 0.00)),
    CONSTRAINT "CK_ProductVendor_MaxOrderQty" CHECK ((maxorderqty >= 1)),
    CONSTRAINT "CK_ProductVendor_MinOrderQty" CHECK ((minorderqty >= 1)),
    CONSTRAINT "CK_ProductVendor_OnOrderQty" CHECK ((onorderqty >= 0)),
    CONSTRAINT "CK_ProductVendor_StandardPrice" CHECK ((standardprice > 0.00))
);


CREATE TABLE purchasing.shipmethod (
    shipmethodid integer NOT NULL,
    name public."Name" NOT NULL,
    shipbase numeric DEFAULT 0.00 NOT NULL,
    shiprate numeric DEFAULT 0.00 NOT NULL,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_ShipMethod_ShipBase" CHECK ((shipbase > 0.00)),
    CONSTRAINT "CK_ShipMethod_ShipRate" CHECK ((shiprate > 0.00))
);


CREATE TABLE purchasing.vendor (
    businessentityid integer NOT NULL,
    accountnumber public."AccountNumber" NOT NULL,
    name public."Name" NOT NULL,
    creditrating smallint NOT NULL,
    preferredvendorstatus public."Flag" DEFAULT true NOT NULL,
    activeflag public."Flag" DEFAULT true NOT NULL,
    purchasingwebserviceurl character varying(1024),
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_Vendor_CreditRating" CHECK (((creditrating >= 1) AND (creditrating <= 5)))
);


CREATE TABLE sales.customer (
    customerid integer NOT NULL,
    personid integer,
    storeid integer,
    territoryid integer,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE sales.creditcard (
    creditcardid integer NOT NULL,
    cardtype character varying(50) NOT NULL,
    cardnumber character varying(25) NOT NULL,
    expmonth smallint NOT NULL,
    expyear smallint NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE sales.currencyrate (
    currencyrateid integer NOT NULL,
    currencyratedate timestamp without time zone NOT NULL,
    fromcurrencycode character(3) NOT NULL,
    tocurrencycode character(3) NOT NULL,
    averagerate numeric NOT NULL,
    endofdayrate numeric NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE sales.countryregioncurrency (
    countryregioncode character varying(3) NOT NULL,
    currencycode character(3) NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE sales.currency (
    currencycode character(3) NOT NULL,
    name public."Name" NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE sales.personcreditcard (
    businessentityid integer NOT NULL,
    creditcardid integer NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE sales.store (
    businessentityid integer NOT NULL,
    name public."Name" NOT NULL,
    salespersonid integer,
    demographics xml,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE sales.shoppingcartitem (
    shoppingcartitemid integer NOT NULL,
    shoppingcartid character varying(50) NOT NULL,
    quantity integer DEFAULT 1 NOT NULL,
    productid integer NOT NULL,
    datecreated timestamp without time zone DEFAULT now() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_ShoppingCartItem_Quantity" CHECK ((quantity >= 1))
);


CREATE TABLE sales.specialoffer (
    specialofferid integer NOT NULL,
    description character varying(255) NOT NULL,
    discountpct numeric DEFAULT 0.00 NOT NULL,
    type character varying(50) NOT NULL,
    category character varying(50) NOT NULL,
    startdate timestamp without time zone NOT NULL,
    enddate timestamp without time zone NOT NULL,
    minqty integer DEFAULT 0 NOT NULL,
    maxqty integer,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_SpecialOffer_DiscountPct" CHECK ((discountpct >= 0.00)),
    CONSTRAINT "CK_SpecialOffer_EndDate" CHECK ((enddate >= startdate)),
    CONSTRAINT "CK_SpecialOffer_MaxQty" CHECK ((maxqty >= 0)),
    CONSTRAINT "CK_SpecialOffer_MinQty" CHECK ((minqty >= 0))
);


CREATE TABLE sales.salesorderdetail (
    salesorderid integer NOT NULL,
    salesorderdetailid integer NOT NULL,
    carriertrackingnumber character varying(25),
    orderqty smallint NOT NULL,
    productid integer NOT NULL,
    specialofferid integer NOT NULL,
    unitprice numeric NOT NULL,
    unitpricediscount numeric DEFAULT 0.0 NOT NULL,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_SalesOrderDetail_OrderQty" CHECK ((orderqty > 0)),
    CONSTRAINT "CK_SalesOrderDetail_UnitPrice" CHECK ((unitprice >= 0.00)),
    CONSTRAINT "CK_SalesOrderDetail_UnitPriceDiscount" CHECK ((unitpricediscount >= 0.00))
);


CREATE TABLE sales.salesorderheader (
    salesorderid integer NOT NULL,
    revisionnumber smallint DEFAULT 0 NOT NULL,
    orderdate timestamp without time zone DEFAULT now() NOT NULL,
    duedate timestamp without time zone NOT NULL,
    shipdate timestamp without time zone,
    status smallint DEFAULT 1 NOT NULL,
    onlineorderflag public."Flag" DEFAULT true NOT NULL,
    purchaseordernumber public."OrderNumber",
    accountnumber public."AccountNumber",
    customerid integer NOT NULL,
    salespersonid integer,
    territoryid integer,
    billtoaddressid integer NOT NULL,
    shiptoaddressid integer NOT NULL,
    shipmethodid integer NOT NULL,
    creditcardid integer,
    creditcardapprovalcode character varying(15),
    currencyrateid integer,
    subtotal numeric DEFAULT 0.00 NOT NULL,
    taxamt numeric DEFAULT 0.00 NOT NULL,
    freight numeric DEFAULT 0.00 NOT NULL,
    totaldue numeric,
    comment character varying(128),
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_SalesOrderHeader_DueDate" CHECK ((duedate >= orderdate)),
    CONSTRAINT "CK_SalesOrderHeader_Freight" CHECK ((freight >= 0.00)),
    CONSTRAINT "CK_SalesOrderHeader_ShipDate" CHECK (((shipdate >= orderdate) OR (shipdate IS NULL))),
    CONSTRAINT "CK_SalesOrderHeader_Status" CHECK (((status >= 0) AND (status <= 8))),
    CONSTRAINT "CK_SalesOrderHeader_SubTotal" CHECK ((subtotal >= 0.00)),
    CONSTRAINT "CK_SalesOrderHeader_TaxAmt" CHECK ((taxamt >= 0.00))
);


CREATE TABLE sales.salesorderheadersalesreason (
    salesorderid integer NOT NULL,
    salesreasonid integer NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE sales.specialofferproduct (
    specialofferid integer NOT NULL,
    productid integer NOT NULL,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE sales.salesperson (
    businessentityid integer NOT NULL,
    territoryid integer,
    salesquota numeric,
    bonus numeric DEFAULT 0.00 NOT NULL,
    commissionpct numeric DEFAULT 0.00 NOT NULL,
    salesytd numeric DEFAULT 0.00 NOT NULL,
    saleslastyear numeric DEFAULT 0.00 NOT NULL,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_SalesPerson_Bonus" CHECK ((bonus >= 0.00)),
    CONSTRAINT "CK_SalesPerson_CommissionPct" CHECK ((commissionpct >= 0.00)),
    CONSTRAINT "CK_SalesPerson_SalesLastYear" CHECK ((saleslastyear >= 0.00)),
    CONSTRAINT "CK_SalesPerson_SalesQuota" CHECK ((salesquota > 0.00)),
    CONSTRAINT "CK_SalesPerson_SalesYTD" CHECK ((salesytd >= 0.00))
);


CREATE TABLE sales.salespersonquotahistory (
    businessentityid integer NOT NULL,
    quotadate timestamp without time zone NOT NULL,
    salesquota numeric NOT NULL,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_SalesPersonQuotaHistory_SalesQuota" CHECK ((salesquota > 0.00))
);


CREATE TABLE sales.salesreason (
    salesreasonid integer NOT NULL,
    name public."Name" NOT NULL,
    reasontype public."Name" NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL
);


CREATE TABLE sales.salesterritory (
    territoryid integer NOT NULL,
    name public."Name" NOT NULL,
    countryregioncode character varying(3) NOT NULL,
    "group" character varying(50) NOT NULL,
    salesytd numeric DEFAULT 0.00 NOT NULL,
    saleslastyear numeric DEFAULT 0.00 NOT NULL,
    costytd numeric DEFAULT 0.00 NOT NULL,
    costlastyear numeric DEFAULT 0.00 NOT NULL,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_SalesTerritory_CostLastYear" CHECK ((costlastyear >= 0.00)),
    CONSTRAINT "CK_SalesTerritory_CostYTD" CHECK ((costytd >= 0.00)),
    CONSTRAINT "CK_SalesTerritory_SalesLastYear" CHECK ((saleslastyear >= 0.00)),
    CONSTRAINT "CK_SalesTerritory_SalesYTD" CHECK ((salesytd >= 0.00))
);


CREATE TABLE sales.salesterritoryhistory (
    businessentityid integer NOT NULL,
    territoryid integer NOT NULL,
    startdate timestamp without time zone NOT NULL,
    enddate timestamp without time zone,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_SalesTerritoryHistory_EndDate" CHECK (((enddate >= startdate) OR (enddate IS NULL)))
);


CREATE TABLE sales.salestaxrate (
    salestaxrateid integer NOT NULL,
    stateprovinceid integer NOT NULL,
    taxtype smallint NOT NULL,
    taxrate numeric DEFAULT 0.00 NOT NULL,
    name public."Name" NOT NULL,
    rowguid uuid DEFAULT public.uuid_generate_v1() NOT NULL,
    modifieddate timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT "CK_SalesTaxRate_TaxType" CHECK (((taxtype >= 1) AND (taxtype <= 3)))
);


ALTER TABLE ONLY humanresources.department ALTER COLUMN departmentid SET DEFAULT nextval('humanresources.department_departmentid_seq'::regclass);


ALTER TABLE ONLY humanresources.jobcandidate ALTER COLUMN jobcandidateid SET DEFAULT nextval('humanresources.jobcandidate_jobcandidateid_seq'::regclass);


ALTER TABLE ONLY humanresources.shift ALTER COLUMN shiftid SET DEFAULT nextval('humanresources.shift_shiftid_seq'::regclass);


ALTER TABLE ONLY person.address ALTER COLUMN addressid SET DEFAULT nextval('person.address_addressid_seq'::regclass);


ALTER TABLE ONLY person.addresstype ALTER COLUMN addresstypeid SET DEFAULT nextval('person.addresstype_addresstypeid_seq'::regclass);


ALTER TABLE ONLY person.businessentity ALTER COLUMN businessentityid SET DEFAULT nextval('person.businessentity_businessentityid_seq'::regclass);


ALTER TABLE ONLY person.contacttype ALTER COLUMN contacttypeid SET DEFAULT nextval('person.contacttype_contacttypeid_seq'::regclass);


ALTER TABLE ONLY person.emailaddress ALTER COLUMN emailaddressid SET DEFAULT nextval('person.emailaddress_emailaddressid_seq'::regclass);


ALTER TABLE ONLY person.phonenumbertype ALTER COLUMN phonenumbertypeid SET DEFAULT nextval('person.phonenumbertype_phonenumbertypeid_seq'::regclass);


ALTER TABLE ONLY person.stateprovince ALTER COLUMN stateprovinceid SET DEFAULT nextval('person.stateprovince_stateprovinceid_seq'::regclass);


ALTER TABLE ONLY production.billofmaterials ALTER COLUMN billofmaterialsid SET DEFAULT nextval('production.billofmaterials_billofmaterialsid_seq'::regclass);


ALTER TABLE ONLY production.illustration ALTER COLUMN illustrationid SET DEFAULT nextval('production.illustration_illustrationid_seq'::regclass);


ALTER TABLE ONLY production.location ALTER COLUMN locationid SET DEFAULT nextval('production.location_locationid_seq'::regclass);


ALTER TABLE ONLY production.product ALTER COLUMN productid SET DEFAULT nextval('production.product_productid_seq'::regclass);


ALTER TABLE ONLY production.productcategory ALTER COLUMN productcategoryid SET DEFAULT nextval('production.productcategory_productcategoryid_seq'::regclass);


ALTER TABLE ONLY production.productdescription ALTER COLUMN productdescriptionid SET DEFAULT nextval('production.productdescription_productdescriptionid_seq'::regclass);


ALTER TABLE ONLY production.productmodel ALTER COLUMN productmodelid SET DEFAULT nextval('production.productmodel_productmodelid_seq'::regclass);


ALTER TABLE ONLY production.productphoto ALTER COLUMN productphotoid SET DEFAULT nextval('production.productphoto_productphotoid_seq'::regclass);


ALTER TABLE ONLY production.productreview ALTER COLUMN productreviewid SET DEFAULT nextval('production.productreview_productreviewid_seq'::regclass);


ALTER TABLE ONLY production.productsubcategory ALTER COLUMN productsubcategoryid SET DEFAULT nextval('production.productsubcategory_productsubcategoryid_seq'::regclass);


ALTER TABLE ONLY production.scrapreason ALTER COLUMN scrapreasonid SET DEFAULT nextval('production.scrapreason_scrapreasonid_seq'::regclass);


ALTER TABLE ONLY production.transactionhistory ALTER COLUMN transactionid SET DEFAULT nextval('production.transactionhistory_transactionid_seq'::regclass);


ALTER TABLE ONLY production.workorder ALTER COLUMN workorderid SET DEFAULT nextval('production.workorder_workorderid_seq'::regclass);


ALTER TABLE ONLY purchasing.purchaseorderdetail ALTER COLUMN purchaseorderdetailid SET DEFAULT nextval('purchasing.purchaseorderdetail_purchaseorderdetailid_seq'::regclass);


ALTER TABLE ONLY purchasing.purchaseorderheader ALTER COLUMN purchaseorderid SET DEFAULT nextval('purchasing.purchaseorderheader_purchaseorderid_seq'::regclass);


ALTER TABLE ONLY purchasing.shipmethod ALTER COLUMN shipmethodid SET DEFAULT nextval('purchasing.shipmethod_shipmethodid_seq'::regclass);


ALTER TABLE ONLY sales.creditcard ALTER COLUMN creditcardid SET DEFAULT nextval('sales.creditcard_creditcardid_seq'::regclass);


ALTER TABLE ONLY sales.currencyrate ALTER COLUMN currencyrateid SET DEFAULT nextval('sales.currencyrate_currencyrateid_seq'::regclass);


ALTER TABLE ONLY sales.customer ALTER COLUMN customerid SET DEFAULT nextval('sales.customer_customerid_seq'::regclass);


ALTER TABLE ONLY sales.salesorderdetail ALTER COLUMN salesorderdetailid SET DEFAULT nextval('sales.salesorderdetail_salesorderdetailid_seq'::regclass);


ALTER TABLE ONLY sales.salesorderheader ALTER COLUMN salesorderid SET DEFAULT nextval('sales.salesorderheader_salesorderid_seq'::regclass);


ALTER TABLE ONLY sales.salesreason ALTER COLUMN salesreasonid SET DEFAULT nextval('sales.salesreason_salesreasonid_seq'::regclass);


ALTER TABLE ONLY sales.salestaxrate ALTER COLUMN salestaxrateid SET DEFAULT nextval('sales.salestaxrate_salestaxrateid_seq'::regclass);


ALTER TABLE ONLY sales.salesterritory ALTER COLUMN territoryid SET DEFAULT nextval('sales.salesterritory_territoryid_seq'::regclass);


ALTER TABLE ONLY sales.shoppingcartitem ALTER COLUMN shoppingcartitemid SET DEFAULT nextval('sales.shoppingcartitem_shoppingcartitemid_seq'::regclass);


ALTER TABLE ONLY sales.specialoffer ALTER COLUMN specialofferid SET DEFAULT nextval('sales.specialoffer_specialofferid_seq'::regclass);


ALTER TABLE ONLY humanresources.department
    ADD CONSTRAINT "PK_Department_DepartmentID" PRIMARY KEY (departmentid);


ALTER TABLE ONLY humanresources.employeedepartmenthistory
    ADD CONSTRAINT "PK_EmployeeDepartmentHistory_BusinessEntityID_StartDate_Departm" PRIMARY KEY (businessentityid, startdate, departmentid, shiftid);


ALTER TABLE ONLY humanresources.employeepayhistory
    ADD CONSTRAINT "PK_EmployeePayHistory_BusinessEntityID_RateChangeDate" PRIMARY KEY (businessentityid, ratechangedate);



ALTER TABLE ONLY humanresources.employee
    ADD CONSTRAINT "PK_Employee_BusinessEntityID" PRIMARY KEY (businessentityid);



ALTER TABLE ONLY humanresources.jobcandidate
    ADD CONSTRAINT "PK_JobCandidate_JobCandidateID" PRIMARY KEY (jobcandidateid);



ALTER TABLE ONLY humanresources.shift
    ADD CONSTRAINT "PK_Shift_ShiftID" PRIMARY KEY (shiftid);



ALTER TABLE ONLY person.addresstype
    ADD CONSTRAINT "PK_AddressType_AddressTypeID" PRIMARY KEY (addresstypeid);



ALTER TABLE ONLY person.address
    ADD CONSTRAINT "PK_Address_AddressID" PRIMARY KEY (addressid);



ALTER TABLE ONLY person.businessentityaddress
    ADD CONSTRAINT "PK_BusinessEntityAddress_BusinessEntityID_AddressID_AddressType" PRIMARY KEY (businessentityid, addressid, addresstypeid);



ALTER TABLE ONLY person.businessentitycontact
    ADD CONSTRAINT "PK_BusinessEntityContact_BusinessEntityID_PersonID_ContactTypeI" PRIMARY KEY (businessentityid, personid, contacttypeid);



ALTER TABLE ONLY person.businessentity
    ADD CONSTRAINT "PK_BusinessEntity_BusinessEntityID" PRIMARY KEY (businessentityid);



ALTER TABLE ONLY person.contacttype
    ADD CONSTRAINT "PK_ContactType_ContactTypeID" PRIMARY KEY (contacttypeid);



ALTER TABLE ONLY person.countryregion
    ADD CONSTRAINT "PK_CountryRegion_CountryRegionCode" PRIMARY KEY (countryregioncode);



ALTER TABLE ONLY person.emailaddress
    ADD CONSTRAINT "PK_EmailAddress_BusinessEntityID_EmailAddressID" PRIMARY KEY (businessentityid, emailaddressid);



ALTER TABLE ONLY person.password
    ADD CONSTRAINT "PK_Password_BusinessEntityID" PRIMARY KEY (businessentityid);



ALTER TABLE ONLY person.personphone
    ADD CONSTRAINT "PK_PersonPhone_BusinessEntityID_PhoneNumber_PhoneNumberTypeID" PRIMARY KEY (businessentityid, phonenumber, phonenumbertypeid);



ALTER TABLE ONLY person.person
    ADD CONSTRAINT "PK_Person_BusinessEntityID" PRIMARY KEY (businessentityid);



ALTER TABLE ONLY person.phonenumbertype
    ADD CONSTRAINT "PK_PhoneNumberType_PhoneNumberTypeID" PRIMARY KEY (phonenumbertypeid);



ALTER TABLE ONLY person.stateprovince
    ADD CONSTRAINT "PK_StateProvince_StateProvinceID" PRIMARY KEY (stateprovinceid);



ALTER TABLE ONLY production.billofmaterials
    ADD CONSTRAINT "PK_BillOfMaterials_BillOfMaterialsID" PRIMARY KEY (billofmaterialsid);


ALTER TABLE ONLY production.culture
    ADD CONSTRAINT "PK_Culture_CultureID" PRIMARY KEY (cultureid);



ALTER TABLE ONLY production.document
    ADD CONSTRAINT "PK_Document_DocumentNode" PRIMARY KEY (documentnode);



ALTER TABLE ONLY production.illustration
    ADD CONSTRAINT "PK_Illustration_IllustrationID" PRIMARY KEY (illustrationid);



ALTER TABLE ONLY production.location
    ADD CONSTRAINT "PK_Location_LocationID" PRIMARY KEY (locationid);



ALTER TABLE ONLY production.productcategory
    ADD CONSTRAINT "PK_ProductCategory_ProductCategoryID" PRIMARY KEY (productcategoryid);



ALTER TABLE ONLY production.productcosthistory
    ADD CONSTRAINT "PK_ProductCostHistory_ProductID_StartDate" PRIMARY KEY (productid, startdate);



ALTER TABLE ONLY production.productdescription
    ADD CONSTRAINT "PK_ProductDescription_ProductDescriptionID" PRIMARY KEY (productdescriptionid);



ALTER TABLE ONLY production.productdocument
    ADD CONSTRAINT "PK_ProductDocument_ProductID_DocumentNode" PRIMARY KEY (productid, documentnode);



ALTER TABLE ONLY production.productinventory
    ADD CONSTRAINT "PK_ProductInventory_ProductID_LocationID" PRIMARY KEY (productid, locationid);



ALTER TABLE ONLY production.productlistpricehistory
    ADD CONSTRAINT "PK_ProductListPriceHistory_ProductID_StartDate" PRIMARY KEY (productid, startdate);



ALTER TABLE ONLY production.productmodelillustration
    ADD CONSTRAINT "PK_ProductModelIllustration_ProductModelID_IllustrationID" PRIMARY KEY (productmodelid, illustrationid);



ALTER TABLE ONLY production.productmodelproductdescriptionculture
    ADD CONSTRAINT "PK_ProductModelProductDescriptionCulture_ProductModelID_Product" PRIMARY KEY (productmodelid, productdescriptionid, cultureid);



ALTER TABLE ONLY production.productmodel
    ADD CONSTRAINT "PK_ProductModel_ProductModelID" PRIMARY KEY (productmodelid);



ALTER TABLE ONLY production.productphoto
    ADD CONSTRAINT "PK_ProductPhoto_ProductPhotoID" PRIMARY KEY (productphotoid);



ALTER TABLE ONLY production.productproductphoto
    ADD CONSTRAINT "PK_ProductProductPhoto_ProductID_ProductPhotoID" PRIMARY KEY (productid, productphotoid);


ALTER TABLE ONLY production.productreview
    ADD CONSTRAINT "PK_ProductReview_ProductReviewID" PRIMARY KEY (productreviewid);



ALTER TABLE ONLY production.productsubcategory
    ADD CONSTRAINT "PK_ProductSubcategory_ProductSubcategoryID" PRIMARY KEY (productsubcategoryid);



ALTER TABLE ONLY production.product
    ADD CONSTRAINT "PK_Product_ProductID" PRIMARY KEY (productid);



ALTER TABLE ONLY production.scrapreason
    ADD CONSTRAINT "PK_ScrapReason_ScrapReasonID" PRIMARY KEY (scrapreasonid);



ALTER TABLE ONLY production.transactionhistoryarchive
    ADD CONSTRAINT "PK_TransactionHistoryArchive_TransactionID" PRIMARY KEY (transactionid);



ALTER TABLE ONLY production.transactionhistory
    ADD CONSTRAINT "PK_TransactionHistory_TransactionID" PRIMARY KEY (transactionid);



ALTER TABLE ONLY production.unitmeasure
    ADD CONSTRAINT "PK_UnitMeasure_UnitMeasureCode" PRIMARY KEY (unitmeasurecode);



ALTER TABLE ONLY production.workorderrouting
    ADD CONSTRAINT "PK_WorkOrderRouting_WorkOrderID_ProductID_OperationSequence" PRIMARY KEY (workorderid, productid, operationsequence);



ALTER TABLE ONLY production.workorder
    ADD CONSTRAINT "PK_WorkOrder_WorkOrderID" PRIMARY KEY (workorderid);



ALTER TABLE ONLY production.document
    ADD CONSTRAINT document_rowguid_key UNIQUE (rowguid);


ALTER TABLE ONLY purchasing.productvendor
    ADD CONSTRAINT "PK_ProductVendor_ProductID_BusinessEntityID" PRIMARY KEY (productid, businessentityid);



ALTER TABLE ONLY purchasing.purchaseorderdetail
    ADD CONSTRAINT "PK_PurchaseOrderDetail_PurchaseOrderID_PurchaseOrderDetailID" PRIMARY KEY (purchaseorderid, purchaseorderdetailid);



ALTER TABLE ONLY purchasing.purchaseorderheader
    ADD CONSTRAINT "PK_PurchaseOrderHeader_PurchaseOrderID" PRIMARY KEY (purchaseorderid);



ALTER TABLE ONLY purchasing.shipmethod
    ADD CONSTRAINT "PK_ShipMethod_ShipMethodID" PRIMARY KEY (shipmethodid);



ALTER TABLE ONLY purchasing.vendor
    ADD CONSTRAINT "PK_Vendor_BusinessEntityID" PRIMARY KEY (businessentityid);



ALTER TABLE ONLY sales.countryregioncurrency
    ADD CONSTRAINT "PK_CountryRegionCurrency_CountryRegionCode_CurrencyCode" PRIMARY KEY (countryregioncode, currencycode);



ALTER TABLE ONLY sales.creditcard
    ADD CONSTRAINT "PK_CreditCard_CreditCardID" PRIMARY KEY (creditcardid);



ALTER TABLE ONLY sales.currencyrate
    ADD CONSTRAINT "PK_CurrencyRate_CurrencyRateID" PRIMARY KEY (currencyrateid);



ALTER TABLE ONLY sales.currency
    ADD CONSTRAINT "PK_Currency_CurrencyCode" PRIMARY KEY (currencycode);



ALTER TABLE ONLY sales.customer
    ADD CONSTRAINT "PK_Customer_CustomerID" PRIMARY KEY (customerid);



ALTER TABLE ONLY sales.personcreditcard
    ADD CONSTRAINT "PK_PersonCreditCard_BusinessEntityID_CreditCardID" PRIMARY KEY (businessentityid, creditcardid);



ALTER TABLE ONLY sales.salesorderdetail
    ADD CONSTRAINT "PK_SalesOrderDetail_SalesOrderID_SalesOrderDetailID" PRIMARY KEY (salesorderid, salesorderdetailid);



ALTER TABLE ONLY sales.salesorderheadersalesreason
    ADD CONSTRAINT "PK_SalesOrderHeaderSalesReason_SalesOrderID_SalesReasonID" PRIMARY KEY (salesorderid, salesreasonid);



ALTER TABLE ONLY sales.salesorderheader
    ADD CONSTRAINT "PK_SalesOrderHeader_SalesOrderID" PRIMARY KEY (salesorderid);



ALTER TABLE ONLY sales.salespersonquotahistory
    ADD CONSTRAINT "PK_SalesPersonQuotaHistory_BusinessEntityID_QuotaDate" PRIMARY KEY (businessentityid, quotadate);



ALTER TABLE ONLY sales.salesperson
    ADD CONSTRAINT "PK_SalesPerson_BusinessEntityID" PRIMARY KEY (businessentityid);



ALTER TABLE ONLY sales.salesreason
    ADD CONSTRAINT "PK_SalesReason_SalesReasonID" PRIMARY KEY (salesreasonid);



ALTER TABLE ONLY sales.salestaxrate
    ADD CONSTRAINT "PK_SalesTaxRate_SalesTaxRateID" PRIMARY KEY (salestaxrateid);



ALTER TABLE ONLY sales.salesterritoryhistory
    ADD CONSTRAINT "PK_SalesTerritoryHistory_BusinessEntityID_StartDate_TerritoryID" PRIMARY KEY (businessentityid, startdate, territoryid);



ALTER TABLE ONLY sales.salesterritory
    ADD CONSTRAINT "PK_SalesTerritory_TerritoryID" PRIMARY KEY (territoryid);



ALTER TABLE ONLY sales.shoppingcartitem
    ADD CONSTRAINT "PK_ShoppingCartItem_ShoppingCartItemID" PRIMARY KEY (shoppingcartitemid);



ALTER TABLE ONLY sales.specialofferproduct
    ADD CONSTRAINT "PK_SpecialOfferProduct_SpecialOfferID_ProductID" PRIMARY KEY (specialofferid, productid);



ALTER TABLE ONLY sales.specialoffer
    ADD CONSTRAINT "PK_SpecialOffer_SpecialOfferID" PRIMARY KEY (specialofferid);



ALTER TABLE ONLY sales.store
    ADD CONSTRAINT "PK_Store_BusinessEntityID" PRIMARY KEY (businessentityid);



ALTER TABLE ONLY humanresources.employeedepartmenthistory
    ADD CONSTRAINT "FK_EmployeeDepartmentHistory_Department_DepartmentID" FOREIGN KEY (departmentid) REFERENCES humanresources.department(departmentid);


ALTER TABLE ONLY humanresources.employeedepartmenthistory
    ADD CONSTRAINT "FK_EmployeeDepartmentHistory_Employee_BusinessEntityID" FOREIGN KEY (businessentityid) REFERENCES humanresources.employee(businessentityid);


ALTER TABLE ONLY humanresources.employeedepartmenthistory
    ADD CONSTRAINT "FK_EmployeeDepartmentHistory_Shift_ShiftID" FOREIGN KEY (shiftid) REFERENCES humanresources.shift(shiftid);


ALTER TABLE ONLY humanresources.employeepayhistory
    ADD CONSTRAINT "FK_EmployeePayHistory_Employee_BusinessEntityID" FOREIGN KEY (businessentityid) REFERENCES humanresources.employee(businessentityid);


ALTER TABLE ONLY humanresources.employee
    ADD CONSTRAINT "FK_Employee_Person_BusinessEntityID" FOREIGN KEY (businessentityid) REFERENCES person.person(businessentityid);


ALTER TABLE ONLY humanresources.jobcandidate
    ADD CONSTRAINT "FK_JobCandidate_Employee_BusinessEntityID" FOREIGN KEY (businessentityid) REFERENCES humanresources.employee(businessentityid);


ALTER TABLE ONLY person.address
    ADD CONSTRAINT "FK_Address_StateProvince_StateProvinceID" FOREIGN KEY (stateprovinceid) REFERENCES person.stateprovince(stateprovinceid);


ALTER TABLE ONLY person.businessentityaddress
    ADD CONSTRAINT "FK_BusinessEntityAddress_AddressType_AddressTypeID" FOREIGN KEY (addresstypeid) REFERENCES person.addresstype(addresstypeid);


ALTER TABLE ONLY person.businessentityaddress
    ADD CONSTRAINT "FK_BusinessEntityAddress_Address_AddressID" FOREIGN KEY (addressid) REFERENCES person.address(addressid);


ALTER TABLE ONLY person.businessentityaddress
    ADD CONSTRAINT "FK_BusinessEntityAddress_BusinessEntity_BusinessEntityID" FOREIGN KEY (businessentityid) REFERENCES person.businessentity(businessentityid);


ALTER TABLE ONLY person.businessentitycontact
    ADD CONSTRAINT "FK_BusinessEntityContact_BusinessEntity_BusinessEntityID" FOREIGN KEY (businessentityid) REFERENCES person.businessentity(businessentityid);


ALTER TABLE ONLY person.businessentitycontact
    ADD CONSTRAINT "FK_BusinessEntityContact_ContactType_ContactTypeID" FOREIGN KEY (contacttypeid) REFERENCES person.contacttype(contacttypeid);


ALTER TABLE ONLY person.businessentitycontact
    ADD CONSTRAINT "FK_BusinessEntityContact_Person_PersonID" FOREIGN KEY (personid) REFERENCES person.person(businessentityid);


ALTER TABLE ONLY person.emailaddress
    ADD CONSTRAINT "FK_EmailAddress_Person_BusinessEntityID" FOREIGN KEY (businessentityid) REFERENCES person.person(businessentityid);


ALTER TABLE ONLY person.password
    ADD CONSTRAINT "FK_Password_Person_BusinessEntityID" FOREIGN KEY (businessentityid) REFERENCES person.person(businessentityid);


ALTER TABLE ONLY person.personphone
    ADD CONSTRAINT "FK_PersonPhone_Person_BusinessEntityID" FOREIGN KEY (businessentityid) REFERENCES person.person(businessentityid);


ALTER TABLE ONLY person.personphone
    ADD CONSTRAINT "FK_PersonPhone_PhoneNumberType_PhoneNumberTypeID" FOREIGN KEY (phonenumbertypeid) REFERENCES person.phonenumbertype(phonenumbertypeid);


ALTER TABLE ONLY person.person
    ADD CONSTRAINT "FK_Person_BusinessEntity_BusinessEntityID" FOREIGN KEY (businessentityid) REFERENCES person.businessentity(businessentityid);


ALTER TABLE ONLY person.stateprovince
    ADD CONSTRAINT "FK_StateProvince_CountryRegion_CountryRegionCode" FOREIGN KEY (countryregioncode) REFERENCES person.countryregion(countryregioncode);


ALTER TABLE ONLY person.stateprovince
    ADD CONSTRAINT "FK_StateProvince_SalesTerritory_TerritoryID" FOREIGN KEY (territoryid) REFERENCES sales.salesterritory(territoryid);


ALTER TABLE ONLY production.billofmaterials
    ADD CONSTRAINT "FK_BillOfMaterials_Product_ComponentID" FOREIGN KEY (componentid) REFERENCES production.product(productid);


ALTER TABLE ONLY production.billofmaterials
    ADD CONSTRAINT "FK_BillOfMaterials_Product_ProductAssemblyID" FOREIGN KEY (productassemblyid) REFERENCES production.product(productid);


ALTER TABLE ONLY production.billofmaterials
    ADD CONSTRAINT "FK_BillOfMaterials_UnitMeasure_UnitMeasureCode" FOREIGN KEY (unitmeasurecode) REFERENCES production.unitmeasure(unitmeasurecode);


ALTER TABLE ONLY production.document
    ADD CONSTRAINT "FK_Document_Employee_Owner" FOREIGN KEY (owner) REFERENCES humanresources.employee(businessentityid);


ALTER TABLE ONLY production.productcosthistory
    ADD CONSTRAINT "FK_ProductCostHistory_Product_ProductID" FOREIGN KEY (productid) REFERENCES production.product(productid);


ALTER TABLE ONLY production.productdocument
    ADD CONSTRAINT "FK_ProductDocument_Document_DocumentNode" FOREIGN KEY (documentnode) REFERENCES production.document(documentnode);


ALTER TABLE ONLY production.productdocument
    ADD CONSTRAINT "FK_ProductDocument_Product_ProductID" FOREIGN KEY (productid) REFERENCES production.product(productid);


ALTER TABLE ONLY production.productinventory
    ADD CONSTRAINT "FK_ProductInventory_Location_LocationID" FOREIGN KEY (locationid) REFERENCES production.location(locationid);


ALTER TABLE ONLY production.productinventory
    ADD CONSTRAINT "FK_ProductInventory_Product_ProductID" FOREIGN KEY (productid) REFERENCES production.product(productid);


ALTER TABLE ONLY production.productlistpricehistory
    ADD CONSTRAINT "FK_ProductListPriceHistory_Product_ProductID" FOREIGN KEY (productid) REFERENCES production.product(productid);


ALTER TABLE ONLY production.productmodelillustration
    ADD CONSTRAINT "FK_ProductModelIllustration_Illustration_IllustrationID" FOREIGN KEY (illustrationid) REFERENCES production.illustration(illustrationid);


ALTER TABLE ONLY production.productmodelillustration
    ADD CONSTRAINT "FK_ProductModelIllustration_ProductModel_ProductModelID" FOREIGN KEY (productmodelid) REFERENCES production.productmodel(productmodelid);


ALTER TABLE ONLY production.productmodelproductdescriptionculture
    ADD CONSTRAINT "FK_ProductModelProductDescriptionCulture_Culture_CultureID" FOREIGN KEY (cultureid) REFERENCES production.culture(cultureid);


ALTER TABLE ONLY production.productmodelproductdescriptionculture
    ADD CONSTRAINT "FK_ProductModelProductDescriptionCulture_ProductDescription_Pro" FOREIGN KEY (productdescriptionid) REFERENCES production.productdescription(productdescriptionid);


ALTER TABLE ONLY production.productmodelproductdescriptionculture
    ADD CONSTRAINT "FK_ProductModelProductDescriptionCulture_ProductModel_ProductMo" FOREIGN KEY (productmodelid) REFERENCES production.productmodel(productmodelid);


ALTER TABLE ONLY production.productproductphoto
    ADD CONSTRAINT "FK_ProductProductPhoto_ProductPhoto_ProductPhotoID" FOREIGN KEY (productphotoid) REFERENCES production.productphoto(productphotoid);


ALTER TABLE ONLY production.productproductphoto
    ADD CONSTRAINT "FK_ProductProductPhoto_Product_ProductID" FOREIGN KEY (productid) REFERENCES production.product(productid);


ALTER TABLE ONLY production.productreview
    ADD CONSTRAINT "FK_ProductReview_Product_ProductID" FOREIGN KEY (productid) REFERENCES production.product(productid);


ALTER TABLE ONLY production.productsubcategory
    ADD CONSTRAINT "FK_ProductSubcategory_ProductCategory_ProductCategoryID" FOREIGN KEY (productcategoryid) REFERENCES production.productcategory(productcategoryid);


ALTER TABLE ONLY production.product
    ADD CONSTRAINT "FK_Product_ProductModel_ProductModelID" FOREIGN KEY (productmodelid) REFERENCES production.productmodel(productmodelid);


ALTER TABLE ONLY production.product
    ADD CONSTRAINT "FK_Product_ProductSubcategory_ProductSubcategoryID" FOREIGN KEY (productsubcategoryid) REFERENCES production.productsubcategory(productsubcategoryid);


ALTER TABLE ONLY production.product
    ADD CONSTRAINT "FK_Product_UnitMeasure_SizeUnitMeasureCode" FOREIGN KEY (sizeunitmeasurecode) REFERENCES production.unitmeasure(unitmeasurecode);


ALTER TABLE ONLY production.product
    ADD CONSTRAINT "FK_Product_UnitMeasure_WeightUnitMeasureCode" FOREIGN KEY (weightunitmeasurecode) REFERENCES production.unitmeasure(unitmeasurecode);


ALTER TABLE ONLY production.transactionhistory
    ADD CONSTRAINT "FK_TransactionHistory_Product_ProductID" FOREIGN KEY (productid) REFERENCES production.product(productid);


ALTER TABLE ONLY production.workorderrouting
    ADD CONSTRAINT "FK_WorkOrderRouting_Location_LocationID" FOREIGN KEY (locationid) REFERENCES production.location(locationid);


ALTER TABLE ONLY production.workorderrouting
    ADD CONSTRAINT "FK_WorkOrderRouting_WorkOrder_WorkOrderID" FOREIGN KEY (workorderid) REFERENCES production.workorder(workorderid);


ALTER TABLE ONLY production.workorder
    ADD CONSTRAINT "FK_WorkOrder_Product_ProductID" FOREIGN KEY (productid) REFERENCES production.product(productid);


ALTER TABLE ONLY production.workorder
    ADD CONSTRAINT "FK_WorkOrder_ScrapReason_ScrapReasonID" FOREIGN KEY (scrapreasonid) REFERENCES production.scrapreason(scrapreasonid);


ALTER TABLE ONLY purchasing.productvendor
    ADD CONSTRAINT "FK_ProductVendor_Product_ProductID" FOREIGN KEY (productid) REFERENCES production.product(productid);


ALTER TABLE ONLY purchasing.productvendor
    ADD CONSTRAINT "FK_ProductVendor_UnitMeasure_UnitMeasureCode" FOREIGN KEY (unitmeasurecode) REFERENCES production.unitmeasure(unitmeasurecode);


ALTER TABLE ONLY purchasing.productvendor
    ADD CONSTRAINT "FK_ProductVendor_Vendor_BusinessEntityID" FOREIGN KEY (businessentityid) REFERENCES purchasing.vendor(businessentityid);


ALTER TABLE ONLY purchasing.purchaseorderdetail
    ADD CONSTRAINT "FK_PurchaseOrderDetail_Product_ProductID" FOREIGN KEY (productid) REFERENCES production.product(productid);


ALTER TABLE ONLY purchasing.purchaseorderdetail
    ADD CONSTRAINT "FK_PurchaseOrderDetail_PurchaseOrderHeader_PurchaseOrderID" FOREIGN KEY (purchaseorderid) REFERENCES purchasing.purchaseorderheader(purchaseorderid);


ALTER TABLE ONLY purchasing.purchaseorderheader
    ADD CONSTRAINT "FK_PurchaseOrderHeader_Employee_EmployeeID" FOREIGN KEY (employeeid) REFERENCES humanresources.employee(businessentityid);


ALTER TABLE ONLY purchasing.purchaseorderheader
    ADD CONSTRAINT "FK_PurchaseOrderHeader_ShipMethod_ShipMethodID" FOREIGN KEY (shipmethodid) REFERENCES purchasing.shipmethod(shipmethodid);


ALTER TABLE ONLY purchasing.purchaseorderheader
    ADD CONSTRAINT "FK_PurchaseOrderHeader_Vendor_VendorID" FOREIGN KEY (vendorid) REFERENCES purchasing.vendor(businessentityid);


ALTER TABLE ONLY purchasing.vendor
    ADD CONSTRAINT "FK_Vendor_BusinessEntity_BusinessEntityID" FOREIGN KEY (businessentityid) REFERENCES person.businessentity(businessentityid);


ALTER TABLE ONLY sales.countryregioncurrency
    ADD CONSTRAINT "FK_CountryRegionCurrency_CountryRegion_CountryRegionCode" FOREIGN KEY (countryregioncode) REFERENCES person.countryregion(countryregioncode);


ALTER TABLE ONLY sales.countryregioncurrency
    ADD CONSTRAINT "FK_CountryRegionCurrency_Currency_CurrencyCode" FOREIGN KEY (currencycode) REFERENCES sales.currency(currencycode);


ALTER TABLE ONLY sales.currencyrate
    ADD CONSTRAINT "FK_CurrencyRate_Currency_FromCurrencyCode" FOREIGN KEY (fromcurrencycode) REFERENCES sales.currency(currencycode);


ALTER TABLE ONLY sales.currencyrate
    ADD CONSTRAINT "FK_CurrencyRate_Currency_ToCurrencyCode" FOREIGN KEY (tocurrencycode) REFERENCES sales.currency(currencycode);


ALTER TABLE ONLY sales.customer
    ADD CONSTRAINT "FK_Customer_Person_PersonID" FOREIGN KEY (personid) REFERENCES person.person(businessentityid);


ALTER TABLE ONLY sales.customer
    ADD CONSTRAINT "FK_Customer_SalesTerritory_TerritoryID" FOREIGN KEY (territoryid) REFERENCES sales.salesterritory(territoryid);


ALTER TABLE ONLY sales.customer
    ADD CONSTRAINT "FK_Customer_Store_StoreID" FOREIGN KEY (storeid) REFERENCES sales.store(businessentityid);


ALTER TABLE ONLY sales.personcreditcard
    ADD CONSTRAINT "FK_PersonCreditCard_CreditCard_CreditCardID" FOREIGN KEY (creditcardid) REFERENCES sales.creditcard(creditcardid);


ALTER TABLE ONLY sales.personcreditcard
    ADD CONSTRAINT "FK_PersonCreditCard_Person_BusinessEntityID" FOREIGN KEY (businessentityid) REFERENCES person.person(businessentityid);


ALTER TABLE ONLY sales.salesorderdetail
    ADD CONSTRAINT "FK_SalesOrderDetail_SalesOrderHeader_SalesOrderID" FOREIGN KEY (salesorderid) REFERENCES sales.salesorderheader(salesorderid) ON DELETE CASCADE;


ALTER TABLE ONLY sales.salesorderdetail
    ADD CONSTRAINT "FK_SalesOrderDetail_SpecialOfferProduct_SpecialOfferIDProductID" FOREIGN KEY (specialofferid, productid) REFERENCES sales.specialofferproduct(specialofferid, productid);


ALTER TABLE ONLY sales.salesorderheadersalesreason
    ADD CONSTRAINT "FK_SalesOrderHeaderSalesReason_SalesOrderHeader_SalesOrderID" FOREIGN KEY (salesorderid) REFERENCES sales.salesorderheader(salesorderid) ON DELETE CASCADE;


ALTER TABLE ONLY sales.salesorderheadersalesreason
    ADD CONSTRAINT "FK_SalesOrderHeaderSalesReason_SalesReason_SalesReasonID" FOREIGN KEY (salesreasonid) REFERENCES sales.salesreason(salesreasonid);


ALTER TABLE ONLY sales.salesorderheader
    ADD CONSTRAINT "FK_SalesOrderHeader_Address_BillToAddressID" FOREIGN KEY (billtoaddressid) REFERENCES person.address(addressid);


ALTER TABLE ONLY sales.salesorderheader
    ADD CONSTRAINT "FK_SalesOrderHeader_Address_ShipToAddressID" FOREIGN KEY (shiptoaddressid) REFERENCES person.address(addressid);


ALTER TABLE ONLY sales.salesorderheader
    ADD CONSTRAINT "FK_SalesOrderHeader_CreditCard_CreditCardID" FOREIGN KEY (creditcardid) REFERENCES sales.creditcard(creditcardid);


ALTER TABLE ONLY sales.salesorderheader
    ADD CONSTRAINT "FK_SalesOrderHeader_CurrencyRate_CurrencyRateID" FOREIGN KEY (currencyrateid) REFERENCES sales.currencyrate(currencyrateid);


ALTER TABLE ONLY sales.salesorderheader
    ADD CONSTRAINT "FK_SalesOrderHeader_Customer_CustomerID" FOREIGN KEY (customerid) REFERENCES sales.customer(customerid);


ALTER TABLE ONLY sales.salesorderheader
    ADD CONSTRAINT "FK_SalesOrderHeader_SalesPerson_SalesPersonID" FOREIGN KEY (salespersonid) REFERENCES sales.salesperson(businessentityid);


ALTER TABLE ONLY sales.salesorderheader
    ADD CONSTRAINT "FK_SalesOrderHeader_SalesTerritory_TerritoryID" FOREIGN KEY (territoryid) REFERENCES sales.salesterritory(territoryid);


ALTER TABLE ONLY sales.salesorderheader
    ADD CONSTRAINT "FK_SalesOrderHeader_ShipMethod_ShipMethodID" FOREIGN KEY (shipmethodid) REFERENCES purchasing.shipmethod(shipmethodid);


ALTER TABLE ONLY sales.salespersonquotahistory
    ADD CONSTRAINT "FK_SalesPersonQuotaHistory_SalesPerson_BusinessEntityID" FOREIGN KEY (businessentityid) REFERENCES sales.salesperson(businessentityid);


ALTER TABLE ONLY sales.salesperson
    ADD CONSTRAINT "FK_SalesPerson_Employee_BusinessEntityID" FOREIGN KEY (businessentityid) REFERENCES humanresources.employee(businessentityid);


ALTER TABLE ONLY sales.salesperson
    ADD CONSTRAINT "FK_SalesPerson_SalesTerritory_TerritoryID" FOREIGN KEY (territoryid) REFERENCES sales.salesterritory(territoryid);


ALTER TABLE ONLY sales.salestaxrate
    ADD CONSTRAINT "FK_SalesTaxRate_StateProvince_StateProvinceID" FOREIGN KEY (stateprovinceid) REFERENCES person.stateprovince(stateprovinceid);


ALTER TABLE ONLY sales.salesterritoryhistory
    ADD CONSTRAINT "FK_SalesTerritoryHistory_SalesPerson_BusinessEntityID" FOREIGN KEY (businessentityid) REFERENCES sales.salesperson(businessentityid);


ALTER TABLE ONLY sales.salesterritoryhistory
    ADD CONSTRAINT "FK_SalesTerritoryHistory_SalesTerritory_TerritoryID" FOREIGN KEY (territoryid) REFERENCES sales.salesterritory(territoryid);


ALTER TABLE ONLY sales.salesterritory
    ADD CONSTRAINT "FK_SalesTerritory_CountryRegion_CountryRegionCode" FOREIGN KEY (countryregioncode) REFERENCES person.countryregion(countryregioncode);


ALTER TABLE ONLY sales.shoppingcartitem
    ADD CONSTRAINT "FK_ShoppingCartItem_Product_ProductID" FOREIGN KEY (productid) REFERENCES production.product(productid);


ALTER TABLE ONLY sales.specialofferproduct
    ADD CONSTRAINT "FK_SpecialOfferProduct_Product_ProductID" FOREIGN KEY (productid) REFERENCES production.product(productid);


ALTER TABLE ONLY sales.specialofferproduct
    ADD CONSTRAINT "FK_SpecialOfferProduct_SpecialOffer_SpecialOfferID" FOREIGN KEY (specialofferid) REFERENCES sales.specialoffer(specialofferid);


ALTER TABLE ONLY sales.store
    ADD CONSTRAINT "FK_Store_BusinessEntity_BusinessEntityID" FOREIGN KEY (businessentityid) REFERENCES person.businessentity(businessentityid);


ALTER TABLE ONLY sales.store
    ADD CONSTRAINT "FK_Store_SalesPerson_SalesPersonID" FOREIGN KEY (salespersonid) REFERENCES sales.salesperson(businessentityid);



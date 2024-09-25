/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE humanresources.department CLUSTER ON "PK_Department_DepartmentID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE humanresources.employeedepartmenthistory CLUSTER ON "PK_EmployeeDepartmentHistory_BusinessEntityID_StartDate_Departm";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE humanresources.employeepayhistory CLUSTER ON "PK_EmployeePayHistory_BusinessEntityID_RateChangeDate";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE humanresources.employee CLUSTER ON "PK_Employee_BusinessEntityID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE humanresources.jobcandidate CLUSTER ON "PK_JobCandidate_JobCandidateID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE humanresources.shift CLUSTER ON "PK_Shift_ShiftID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE person.addresstype CLUSTER ON "PK_AddressType_AddressTypeID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE person.address CLUSTER ON "PK_Address_AddressID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE person.businessentityaddress CLUSTER ON "PK_BusinessEntityAddress_BusinessEntityID_AddressID_AddressType";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE person.businessentitycontact CLUSTER ON "PK_BusinessEntityContact_BusinessEntityID_PersonID_ContactTypeI";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE person.businessentity CLUSTER ON "PK_BusinessEntity_BusinessEntityID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE person.contacttype CLUSTER ON "PK_ContactType_ContactTypeID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE person.countryregion CLUSTER ON "PK_CountryRegion_CountryRegionCode";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE person.emailaddress CLUSTER ON "PK_EmailAddress_BusinessEntityID_EmailAddressID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE person.password CLUSTER ON "PK_Password_BusinessEntityID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE person.personphone CLUSTER ON "PK_PersonPhone_BusinessEntityID_PhoneNumber_PhoneNumberTypeID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE person.person CLUSTER ON "PK_Person_BusinessEntityID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE person.phonenumbertype CLUSTER ON "PK_PhoneNumberType_PhoneNumberTypeID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE person.stateprovince CLUSTER ON "PK_StateProvince_StateProvinceID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE production.culture CLUSTER ON "PK_Culture_CultureID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE production.document CLUSTER ON "PK_Document_DocumentNode";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE production.illustration CLUSTER ON "PK_Illustration_IllustrationID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE production.location CLUSTER ON "PK_Location_LocationID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE production.productcategory CLUSTER ON "PK_ProductCategory_ProductCategoryID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE production.productcosthistory CLUSTER ON "PK_ProductCostHistory_ProductID_StartDate";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE production.productdescription CLUSTER ON "PK_ProductDescription_ProductDescriptionID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE production.productdocument CLUSTER ON "PK_ProductDocument_ProductID_DocumentNode";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE production.productinventory CLUSTER ON "PK_ProductInventory_ProductID_LocationID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE production.productlistpricehistory CLUSTER ON "PK_ProductListPriceHistory_ProductID_StartDate";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE production.productmodelillustration CLUSTER ON "PK_ProductModelIllustration_ProductModelID_IllustrationID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE production.productmodelproductdescriptionculture CLUSTER ON "PK_ProductModelProductDescriptionCulture_ProductModelID_Product";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE production.productmodel CLUSTER ON "PK_ProductModel_ProductModelID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE production.productphoto CLUSTER ON "PK_ProductPhoto_ProductPhotoID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE production.productreview CLUSTER ON "PK_ProductReview_ProductReviewID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE production.productsubcategory CLUSTER ON "PK_ProductSubcategory_ProductSubcategoryID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE production.product CLUSTER ON "PK_Product_ProductID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE production.scrapreason CLUSTER ON "PK_ScrapReason_ScrapReasonID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE production.transactionhistoryarchive CLUSTER ON "PK_TransactionHistoryArchive_TransactionID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE production.transactionhistory CLUSTER ON "PK_TransactionHistory_TransactionID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE production.unitmeasure CLUSTER ON "PK_UnitMeasure_UnitMeasureCode";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE production.workorderrouting CLUSTER ON "PK_WorkOrderRouting_WorkOrderID_ProductID_OperationSequence";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE production.workorder CLUSTER ON "PK_WorkOrder_WorkOrderID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE purchasing.productvendor CLUSTER ON "PK_ProductVendor_ProductID_BusinessEntityID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE purchasing.purchaseorderdetail CLUSTER ON "PK_PurchaseOrderDetail_PurchaseOrderID_PurchaseOrderDetailID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE purchasing.purchaseorderheader CLUSTER ON "PK_PurchaseOrderHeader_PurchaseOrderID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE purchasing.shipmethod CLUSTER ON "PK_ShipMethod_ShipMethodID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE purchasing.vendor CLUSTER ON "PK_Vendor_BusinessEntityID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE sales.countryregioncurrency CLUSTER ON "PK_CountryRegionCurrency_CountryRegionCode_CurrencyCode";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE sales.creditcard CLUSTER ON "PK_CreditCard_CreditCardID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE sales.currencyrate CLUSTER ON "PK_CurrencyRate_CurrencyRateID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE sales.currency CLUSTER ON "PK_Currency_CurrencyCode";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE sales.customer CLUSTER ON "PK_Customer_CustomerID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE sales.personcreditcard CLUSTER ON "PK_PersonCreditCard_BusinessEntityID_CreditCardID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE sales.salesorderdetail CLUSTER ON "PK_SalesOrderDetail_SalesOrderID_SalesOrderDetailID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE sales.salesorderheadersalesreason CLUSTER ON "PK_SalesOrderHeaderSalesReason_SalesOrderID_SalesReasonID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE sales.salesorderheader CLUSTER ON "PK_SalesOrderHeader_SalesOrderID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE sales.salespersonquotahistory CLUSTER ON "PK_SalesPersonQuotaHistory_BusinessEntityID_QuotaDate";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE sales.salesperson CLUSTER ON "PK_SalesPerson_BusinessEntityID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE sales.salesreason CLUSTER ON "PK_SalesReason_SalesReasonID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE sales.salestaxrate CLUSTER ON "PK_SalesTaxRate_SalesTaxRateID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE sales.salesterritoryhistory CLUSTER ON "PK_SalesTerritoryHistory_BusinessEntityID_StartDate_TerritoryID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE sales.salesterritory CLUSTER ON "PK_SalesTerritory_TerritoryID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE sales.shoppingcartitem CLUSTER ON "PK_ShoppingCartItem_ShoppingCartItemID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE sales.specialofferproduct CLUSTER ON "PK_SpecialOfferProduct_SpecialOfferID_ProductID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE sales.specialoffer CLUSTER ON "PK_SpecialOffer_SpecialOfferID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/table.sql
*/
ALTER TABLE sales.store CLUSTER ON "PK_Store_BusinessEntityID";

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/INDEXES_table.sql
*/
ALTER TABLE person.vstateprovincecountryregion CLUSTER ON ix_vstateprovincecountryregion;

/*
ERROR: ALTER TABLE CLUSTER not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/adventureworks/export-dir/schema/tables/INDEXES_table.sql
*/
ALTER TABLE production.vproductanddescription CLUSTER ON ix_vproductanddescription;


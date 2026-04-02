CREATE SCHEMA "CaseSensitiveSchema";

CREATE DOMAIN "CaseSensitiveSchema"."PostalCode" AS text CONSTRAINT "CheckPostalCode" CHECK (
    (
        (VALUE ~ '^\d{5}$' :: text)
        OR (VALUE ~ '^\d{5}-\d{4}$' :: text)
    )
);

CREATE TYPE "CaseSensitiveSchema"."AddressType" AS (
    "Street" text,
    "City" text,
    "State" text,
    "ZipCode" "CaseSensitiveSchema"."PostalCode"
);


CREATE DOMAIN "CaseSensitiveSchema"."EmailAddress" AS character varying(255) CONSTRAINT "CheckEmailFormat" CHECK (
    (
        (VALUE) :: text ~ '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$' :: text
    )
);

CREATE TYPE "CaseSensitiveSchema"."OrderStatus" AS ENUM (
    'Pending',
    'Processing',
    'Shipped',
    'Delivered',
    'Cancelled'
);

CREATE TYPE "CaseSensitiveSchema"."PersonInfo" AS (
    "FirstName" text,
    "LastName" text,
    "Email" "CaseSensitiveSchema"."EmailAddress",
    "Address" "CaseSensitiveSchema"."AddressType"
);

CREATE DOMAIN "CaseSensitiveSchema"."PositiveInteger" AS integer CONSTRAINT "MustBePositive" CHECK ((VALUE > 0));

CREATE TYPE "CaseSensitiveSchema"."PriorityLevel" AS ENUM (
    'Low',
    'Medium',
    'High',
    'Critical'
);

CREATE FUNCTION "CaseSensitiveSchema"."AuditTriggerFunction"() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF TG_OP = 'DELETE' THEN
INSERT INTO
    "CaseSensitiveSchema"."AuditLogTable" (
        "TableName",
        "Action",
        "RecordID",
        "OldValues"
    )
VALUES
    (
        TG_TABLE_NAME,
        TG_OP,
        OLD."CustomerID",
        row_to_json(OLD)
    );

RETURN OLD;

ELSIF TG_OP = 'UPDATE' THEN
INSERT INTO
    "CaseSensitiveSchema"."AuditLogTable" (
        "TableName",
        "Action",
        "RecordID",
        "OldValues",
        "NewValues"
    )
VALUES
    (
        TG_TABLE_NAME,
        TG_OP,
        NEW."CustomerID",
        row_to_json(OLD),
        row_to_json(NEW)
    );

RETURN NEW;

ELSIF TG_OP = 'INSERT' THEN
INSERT INTO
    "CaseSensitiveSchema"."AuditLogTable" (
        "TableName",
        "Action",
        "RecordID",
        "NewValues"
    )
VALUES
    (
        TG_TABLE_NAME,
        TG_OP,
        NEW."CustomerID",
        row_to_json(NEW)
    );

RETURN NEW;

END IF;

RETURN NULL;

END;

$$;


CREATE FUNCTION "CaseSensitiveSchema"."FormatAddress"("Addr" "CaseSensitiveSchema"."AddressType") RETURNS text LANGUAGE sql IMMUTABLE AS $$
SELECT
    "Addr"."Street" || ', ' || "Addr"."City" || ', ' || "Addr"."State" || ' ' || "Addr"."ZipCode" :: TEXT;

$$;

CREATE TABLE "CaseSensitiveSchema"."CustomerTable" (
    "CustomerID" integer NOT NULL,
    "CustomerName" text NOT NULL,
    "Email" "CaseSensitiveSchema"."EmailAddress" NOT NULL,
    "Address" "CaseSensitiveSchema"."AddressType",
    "CreatedAt" timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE FUNCTION "CaseSensitiveSchema"."GetCustomerFullName"("CustomerID" integer) RETURNS text LANGUAGE sql STABLE AS $$
SELECT
    "CustomerName"
FROM
    "CaseSensitiveSchema"."CustomerTable"
WHERE
    "CustomerID" = "GetCustomerFullName"."CustomerID";

$$;

SET
    default_table_access_method = heap;

CREATE TABLE "CaseSensitiveSchema"."OrderTable" (
    "OrderID" integer NOT NULL,
    "CustomerID" integer,
    "OrderStatus" "CaseSensitiveSchema"."OrderStatus" DEFAULT 'Pending' :: "CaseSensitiveSchema"."OrderStatus",
    "Priority" "CaseSensitiveSchema"."PriorityLevel" DEFAULT 'Medium' :: "CaseSensitiveSchema"."PriorityLevel",
    "TotalAmount" numeric(10, 2) NOT NULL DEFAULT 0.00,
    "OrderDate" timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "CheckPositiveAmount" CHECK (("TotalAmount" >= (0) :: numeric))
);

CREATE FUNCTION "CaseSensitiveSchema"."GetOrdersByStatus"("Status" "CaseSensitiveSchema"."OrderStatus") RETURNS SETOF "CaseSensitiveSchema"."OrderTable" LANGUAGE sql STABLE AS $$
SELECT
    *
FROM
    "CaseSensitiveSchema"."OrderTable"
WHERE
    "OrderStatus" = "GetOrdersByStatus"."Status";

$$;

CREATE FUNCTION "CaseSensitiveSchema"."StatusChangeLogger"() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF OLD."OrderStatus" IS DISTINCT
FROM
    NEW."OrderStatus" THEN
INSERT INTO
    "CaseSensitiveSchema"."AuditLogTable" (
        "TableName",
        "Action",
        "RecordID",
        "OldValues",
        "NewValues"
    )
VALUES
    (
        TG_TABLE_NAME,
        'STATUS_CHANGE',
        NEW."OrderID",
        jsonb_build_object('OrderStatus', OLD."OrderStatus"),
        jsonb_build_object('OrderStatus', NEW."OrderStatus")
    );

END IF;

RETURN NEW;

END;

$$;

CREATE FUNCTION "CaseSensitiveSchema"."SumPositiveState"(state integer, val integer) RETURNS integer LANGUAGE sql IMMUTABLE AS $$
SELECT
    COALESCE(state, 0) + CASE
        WHEN val > 0 THEN val
        ELSE 0
    END;

$$;

CREATE PROCEDURE "CaseSensitiveSchema"."UpdateOrderStatus"(
    IN "OrderID" integer,
    IN "NewStatus" "CaseSensitiveSchema"."OrderStatus"
) LANGUAGE plpgsql AS $$ BEGIN
UPDATE
    "CaseSensitiveSchema"."OrderTable"
SET
    "OrderStatus" = "NewStatus"
WHERE
    "OrderTable"."OrderID" = "UpdateOrderStatus"."OrderID";

END;

$$;



CREATE AGGREGATE "CaseSensitiveSchema"."SumPositive"(integer) (
    SFUNC = "CaseSensitiveSchema"."SumPositiveState",
    STYPE = integer,
    INITCOND = '0'
);

CREATE TABLE "CaseSensitiveSchema"."AuditLogTable" (
    "LogID" integer NOT NULL,
    "TableName" text NOT NULL,
    "Action" text NOT NULL,
    "RecordID" integer,
    "ChangedBy" text,
    "ChangeTime" timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    "OldValues" jsonb,
    "NewValues" jsonb
);

CREATE SEQUENCE "CaseSensitiveSchema"."AuditLogTable_LogID_seq" AS integer START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;

ALTER SEQUENCE "CaseSensitiveSchema"."AuditLogTable_LogID_seq" OWNED BY "CaseSensitiveSchema"."AuditLogTable"."LogID";

CREATE MATERIALIZED VIEW "CaseSensitiveSchema"."CaseSensitiveSchema_mview" AS
SELECT
    "CustomerID",
    "CustomerName",
    "Email",
    "Address",
    "CreatedAt"
FROM
    "CaseSensitiveSchema"."CustomerTable" WITH NO DATA;

CREATE SEQUENCE "CaseSensitiveSchema"."CustomSequence" START WITH 1000 INCREMENT BY 5 MINVALUE 1000 MAXVALUE 999999 CACHE 1;

CREATE VIEW "CaseSensitiveSchema"."CustomerOrderView" AS
SELECT
    c."CustomerID",
    c."CustomerName",
    c."Email",
    o."OrderID",
    o."OrderStatus",
    o."TotalAmount",
    o."OrderDate"
FROM
    (
        "CaseSensitiveSchema"."CustomerTable" c
        LEFT JOIN "CaseSensitiveSchema"."OrderTable" o ON ((c."CustomerID" = o."CustomerID"))
    );

CREATE SEQUENCE "CaseSensitiveSchema"."CustomerTable_CustomerID_seq" AS integer START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;

--
-- Name: CustomerTable_CustomerID_seq; Type: SEQUENCE OWNED BY; Schema: CaseSensitiveSchema; Owner: -
--
ALTER SEQUENCE "CaseSensitiveSchema"."CustomerTable_CustomerID_seq" OWNED BY "CaseSensitiveSchema"."CustomerTable"."CustomerID";

CREATE TABLE public."OrderItemTable" (
    "ItemID" integer NOT NULL,
    "OrderID" integer,
    "ProductName" text NOT NULL,
    "Quantity" "CaseSensitiveSchema"."PositiveInteger" NOT NULL,
    "UnitPrice" numeric(10, 2) NOT NULL
);

CREATE SEQUENCE public."OrderItemTable_ItemID_seq" AS integer START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;

ALTER SEQUENCE public."OrderItemTable_ItemID_seq" OWNED BY public."OrderItemTable"."ItemID";

CREATE VIEW "CaseSensitiveSchema"."OrderSummaryView" AS
SELECT
    o."OrderID",
    c."CustomerName",
    o."OrderStatus",
    o."TotalAmount",
    count(oi."ItemID") AS "ItemCount"
FROM
    (
        (
            "CaseSensitiveSchema"."OrderTable" o
            JOIN "CaseSensitiveSchema"."CustomerTable" c ON ((o."CustomerID" = c."CustomerID"))
        )
        LEFT JOIN public."OrderItemTable" oi ON ((o."OrderID" = oi."OrderID"))
    )
GROUP BY
    o."OrderID",
    c."CustomerName",
    o."OrderStatus",
    o."TotalAmount";

CREATE SEQUENCE "CaseSensitiveSchema"."OrderTable_OrderID_seq" AS integer START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;

ALTER SEQUENCE "CaseSensitiveSchema"."OrderTable_OrderID_seq" OWNED BY "CaseSensitiveSchema"."OrderTable"."OrderID";

ALTER TABLE
    ONLY "CaseSensitiveSchema"."AuditLogTable"
ALTER COLUMN
    "LogID"
SET
    DEFAULT nextval(
        '"CaseSensitiveSchema"."AuditLogTable_LogID_seq"' :: regclass
    );

ALTER TABLE
    ONLY "CaseSensitiveSchema"."CustomerTable"
ALTER COLUMN
    "CustomerID"
SET
    DEFAULT nextval(
        '"CaseSensitiveSchema"."CustomerTable_CustomerID_seq"' :: regclass
    );

ALTER TABLE
    ONLY public."OrderItemTable"
ALTER COLUMN
    "ItemID"
SET
    DEFAULT nextval(
        'public."OrderItemTable_ItemID_seq"' :: regclass
    );

ALTER TABLE
    ONLY "CaseSensitiveSchema"."OrderTable"
ALTER COLUMN
    "OrderID"
SET
    DEFAULT nextval(
        '"CaseSensitiveSchema"."OrderTable_OrderID_seq"' :: regclass
    );

ALTER TABLE
    ONLY "CaseSensitiveSchema"."AuditLogTable"
ADD
    CONSTRAINT "AuditLogTable_pkey" PRIMARY KEY ("LogID");

ALTER TABLE
    ONLY "CaseSensitiveSchema"."CustomerTable"
ADD
    CONSTRAINT "CustomerTable_pkey" PRIMARY KEY ("CustomerID");

ALTER TABLE
    ONLY public."OrderItemTable"
ADD
    CONSTRAINT "OrderItemTable_pkey" PRIMARY KEY ("ItemID");

ALTER TABLE
    ONLY "CaseSensitiveSchema"."OrderTable"
ADD
    CONSTRAINT "OrderTable_pkey" PRIMARY KEY ("OrderID");

CREATE INDEX "CustomerEmailIndex" ON "CaseSensitiveSchema"."CustomerTable" USING btree ("Email");

CREATE UNIQUE INDEX "CustomerEmailUniqueIndex" ON "CaseSensitiveSchema"."CustomerTable" USING btree ("Email");

CREATE INDEX "OrderCustomerIDIndex" ON "CaseSensitiveSchema"."OrderTable" USING btree ("CustomerID");

CREATE INDEX "OrderStatusIndex" ON "CaseSensitiveSchema"."OrderTable" USING btree ("OrderStatus");

CREATE TRIGGER "BeforeOrderUpdateTrigger" BEFORE
UPDATE
    ON "CaseSensitiveSchema"."OrderTable" FOR EACH ROW EXECUTE FUNCTION "CaseSensitiveSchema"."AuditTriggerFunction"();

CREATE TRIGGER "CustomerAuditTrigger"
AFTER
INSERT
    OR DELETE
    OR
UPDATE
    ON "CaseSensitiveSchema"."CustomerTable" FOR EACH ROW EXECUTE FUNCTION "CaseSensitiveSchema"."AuditTriggerFunction"();


CREATE TRIGGER "StatusChangeTrigger"
AFTER
UPDATE
    ON "CaseSensitiveSchema"."OrderTable" FOR EACH ROW
    WHEN (
        (
            old."OrderStatus" IS DISTINCT
            FROM
                new."OrderStatus"
        )
    ) EXECUTE FUNCTION "CaseSensitiveSchema"."StatusChangeLogger"();

ALTER TABLE
    ONLY public."OrderItemTable"
ADD
    CONSTRAINT "OrderItemTable_OrderID_fkey" FOREIGN KEY ("OrderID") REFERENCES "CaseSensitiveSchema"."OrderTable"("OrderID") ON DELETE CASCADE;

ALTER TABLE
    ONLY "CaseSensitiveSchema"."OrderTable"
ADD
    CONSTRAINT "OrderTable_CustomerID_fkey" FOREIGN KEY ("CustomerID") REFERENCES "CaseSensitiveSchema"."CustomerTable"("CustomerID");

INSERT INTO
    "CaseSensitiveSchema"."CustomerTable" ("CustomerName", "Email", "Address", "CreatedAt")
VALUES
    (
        'John Smith',
        'john.smith@example.com',
        ROW('123 Main Street', 'New York', 'NY', '10001') :: "CaseSensitiveSchema"."AddressType",
        '2024-01-15 10:30:00'
    ),
    (
        'Jane Doe',
        'jane.doe@test.com',
        ROW('456 Oak Avenue', 'Los Angeles', 'CA', '90001') :: "CaseSensitiveSchema"."AddressType",
        '2024-01-16 14:20:00'
    ),
    (
        'Bob Johnson',
        'bob.johnson@sample.org',
        ROW('789 Pine Road', 'Chicago', 'IL', '60601') :: "CaseSensitiveSchema"."AddressType",
        '2024-01-17 09:15:00'
    ),
    (
        'Alice Williams',
        'alice.w@example.net',
        ROW('321 Elm Street', 'Houston', 'TX', '77001') :: "CaseSensitiveSchema"."AddressType",
        '2024-01-18 16:45:00'
    ),
    (
        'Charlie Brown',
        'charlie.brown@test.org',
        ROW('654 Maple Drive', 'Phoenix', 'AZ', '85001') :: "CaseSensitiveSchema"."AddressType",
        '2024-01-19 11:00:00'
    ),
    (
        'Diana Prince',
        'diana.prince@example.com',
        ROW('987 Cedar Lane', 'Philadelphia', 'PA', '19101') :: "CaseSensitiveSchema"."AddressType",
        '2024-01-20 13:30:00'
    ),
    (
        'Edward Norton',
        'ed.norton@sample.com',
        ROW(
            '147 Birch Boulevard',
            'San Antonio',
            'TX',
            '78201'
        ) :: "CaseSensitiveSchema"."AddressType",
        '2024-01-21 08:20:00'
    ),
    (
        'Fiona Green',
        'fiona.green@test.net',
        ROW('258 Spruce Court', 'San Diego', 'CA', '92101') :: "CaseSensitiveSchema"."AddressType",
        '2024-01-22 15:10:00'
    ),
    (
        'George Washington',
        'george.washington@example.org',
        ROW(
            '1600 Pennsylvania Avenue',
            'Washington',
            'DC',
            '20500'
        ) :: "CaseSensitiveSchema"."AddressType",
        '2024-01-23 12:00:00'
    ),
    (
        'Helen Keller',
        'helen.keller@test.com',
        ROW('999 Innovation Way', 'Seattle', 'WA', '98101') :: "CaseSensitiveSchema"."AddressType",
        '2024-01-24 09:30:00'
    );

INSERT INTO
    "CaseSensitiveSchema"."OrderTable" (
        "CustomerID",
        "OrderStatus",
        "Priority",
        "TotalAmount",
        "OrderDate"
    )
VALUES
    (
        1,
        'Pending' :: "CaseSensitiveSchema"."OrderStatus",
        'Medium' :: "CaseSensitiveSchema"."PriorityLevel",
        12.00,
        '2024-01-25 10:00:00'
    ),
    (
        1,
        'Processing' :: "CaseSensitiveSchema"."OrderStatus",
        'High' :: "CaseSensitiveSchema"."PriorityLevel",
        12.00,
        '2024-01-26 11:30:00'
    ),
    (
        2,
        'Shipped' :: "CaseSensitiveSchema"."OrderStatus",
        'Medium' :: "CaseSensitiveSchema"."PriorityLevel",
        12.00,
        '2024-01-20 09:15:00'
    ),
    (
        2,
        'Delivered' :: "CaseSensitiveSchema"."OrderStatus",
        'Low' :: "CaseSensitiveSchema"."PriorityLevel",
        12.00,
        '2024-01-15 14:20:00'
    ),
    (
        3,
        'Processing' :: "CaseSensitiveSchema"."OrderStatus",
        'High' :: "CaseSensitiveSchema"."PriorityLevel",
        12.00,
        '2024-01-27 16:45:00'
    ),
    (
        3,
        'Pending' :: "CaseSensitiveSchema"."OrderStatus",
        'Medium' :: "CaseSensitiveSchema"."PriorityLevel",
        12.00,
        '2024-01-28 08:30:00'
    ),
    (
        4,
        'Shipped' :: "CaseSensitiveSchema"."OrderStatus",
        'High' :: "CaseSensitiveSchema"."PriorityLevel",
        12.00,
        '2024-01-22 12:00:00'
    ),
    (
        4,
        'Delivered' :: "CaseSensitiveSchema"."OrderStatus",
        'Low' :: "CaseSensitiveSchema"."PriorityLevel",
        12.00,
        '2024-01-18 10:15:00'
    ),
    (
        5,
        'Cancelled' :: "CaseSensitiveSchema"."OrderStatus",
        'Low' :: "CaseSensitiveSchema"."PriorityLevel",
        12.00,
        '2024-01-19 13:20:00'
    ),
    (
        5,
        'Pending' :: "CaseSensitiveSchema"."OrderStatus",
        'Medium' :: "CaseSensitiveSchema"."PriorityLevel",
        12.00,
        '2024-01-29 09:00:00'
    ),
    (
        6,
        'Processing' :: "CaseSensitiveSchema"."OrderStatus",
        'Critical' :: "CaseSensitiveSchema"."PriorityLevel",
        12.00,
        '2024-01-30 14:30:00'
    ),
    (
        7,
        'Shipped' :: "CaseSensitiveSchema"."OrderStatus",
        'High' :: "CaseSensitiveSchema"."PriorityLevel",
        12.00,
        '2024-01-23 11:45:00'
    ),
    (
        8,
        'Delivered' :: "CaseSensitiveSchema"."OrderStatus",
        'Medium' :: "CaseSensitiveSchema"."PriorityLevel",
        12.00,
        '2024-01-24 15:20:00'
    ),
    (
        9,
        'Pending' :: "CaseSensitiveSchema"."OrderStatus",
        'Low' :: "CaseSensitiveSchema"."PriorityLevel",
        12.00,
        '2024-01-31 10:15:00'
    ),
    (
        10,
        'Processing' :: "CaseSensitiveSchema"."OrderStatus",
        'High' :: "CaseSensitiveSchema"."PriorityLevel",
        12.00,
        '2024-02-01 08:45:00'
    );

-- Order 1 items
INSERT INTO
    public."OrderItemTable" (
        "OrderID",
        "ProductName",
        "Quantity",
        "UnitPrice"
    )
VALUES
    (1, 'Laptop Computer', 1, 1299.99),
    (1, 'Wireless Mouse', 1, 29.99),
    (1, 'USB-C Cable', 2, 19.99);

-- Order 2 items
INSERT INTO
    public."OrderItemTable" (
        "OrderID",
        "ProductName",
        "Quantity",
        "UnitPrice"
    )
VALUES
    (2, 'Smartphone', 1, 899.99),
    (2, 'Phone Case', 1, 24.99),
    (2, 'Screen Protector', 2, 14.99);

-- Order 3 items
INSERT INTO
    public."OrderItemTable" (
        "OrderID",
        "ProductName",
        "Quantity",
        "UnitPrice"
    )
VALUES
    (3, 'Headphones', 1, 199.99),
    (3, 'Audio Cable', 1, 9.99);

-- Order 4 items
INSERT INTO
    public."OrderItemTable" (
        "OrderID",
        "ProductName",
        "Quantity",
        "UnitPrice"
    )
VALUES
    (4, 'Tablet', 1, 499.99),
    (4, 'Stylus Pen', 1, 49.99),
    (4, 'Tablet Stand', 1, 34.99);

-- Order 5 items
INSERT INTO
    public."OrderItemTable" (
        "OrderID",
        "ProductName",
        "Quantity",
        "UnitPrice"
    )
VALUES
    (5, 'Smart Watch', 1, 349.99),
    (5, 'Watch Band', 2, 19.99);

-- Order 6 items
INSERT INTO
    public."OrderItemTable" (
        "OrderID",
        "ProductName",
        "Quantity",
        "UnitPrice"
    )
VALUES
    (6, 'Gaming Console', 1, 499.99),
    (6, 'Controller', 2, 69.99),
    (6, 'Game Disc', 3, 59.99);

-- Order 7 items
INSERT INTO
    public."OrderItemTable" (
        "OrderID",
        "ProductName",
        "Quantity",
        "UnitPrice"
    )
VALUES
    (7, 'Monitor', 1, 299.99),
    (7, 'HDMI Cable', 1, 12.99);

-- Order 8 items
INSERT INTO
    public."OrderItemTable" (
        "OrderID",
        "ProductName",
        "Quantity",
        "UnitPrice"
    )
VALUES
    (8, 'Keyboard', 1, 129.99),
    (8, 'Mouse Pad', 1, 15.99);

-- Order 9 items (cancelled order)
INSERT INTO
    public."OrderItemTable" (
        "OrderID",
        "ProductName",
        "Quantity",
        "UnitPrice"
    )
VALUES
    (9, 'Camera', 1, 799.99);

-- Order 10 items
INSERT INTO
    public."OrderItemTable" (
        "OrderID",
        "ProductName",
        "Quantity",
        "UnitPrice"
    )
VALUES
    (10, 'Printer', 1, 249.99),
    (10, 'Ink Cartridge', 2, 39.99);

-- Order 11 items
INSERT INTO
    public."OrderItemTable" (
        "OrderID",
        "ProductName",
        "Quantity",
        "UnitPrice"
    )
VALUES
    (11, 'Server Rack', 1, 1999.99),
    (11, 'Network Switch', 1, 299.99),
    (11, 'Ethernet Cables', 10, 8.99);

-- Order 12 items
INSERT INTO
    public."OrderItemTable" (
        "OrderID",
        "ProductName",
        "Quantity",
        "UnitPrice"
    )
VALUES
    (12, 'Webcam', 1, 89.99),
    (12, 'Microphone', 1, 79.99);

-- Order 13 items
INSERT INTO
    public."OrderItemTable" (
        "OrderID",
        "ProductName",
        "Quantity",
        "UnitPrice"
    )
VALUES
    (13, 'External Hard Drive', 1, 149.99),
    (13, 'USB Hub', 1, 29.99);

-- Order 14 items
INSERT INTO
    public."OrderItemTable" (
        "OrderID",
        "ProductName",
        "Quantity",
        "UnitPrice"
    )
VALUES
    (14, 'SSD Drive', 1, 199.99),
    (14, 'SATA Cable', 2, 7.99);

-- Order 15 items
INSERT INTO
    public."OrderItemTable" (
        "OrderID",
        "ProductName",
        "Quantity",
        "UnitPrice"
    )
VALUES
    (15, 'RAM Module', 2, 89.99),
    (15, 'Cooling Fan', 1, 24.99);

-- Manual audit log entries (triggers will also create entries automatically)
INSERT INTO
    "CaseSensitiveSchema"."AuditLogTable" (
        "TableName",
        "Action",
        "RecordID",
        "ChangedBy",
        "ChangeTime",
        "OldValues",
        "NewValues"
    )
VALUES
    (
        'CustomerTable',
        'MANUAL_INSERT',
        1,
        'admin',
        '2024-01-15 10:30:00',
        NULL,
        '{"CustomerID": 1, "CustomerName": "John Smith", "Email": "john.smith@example.com"}' :: jsonb
    ),
    (
        'OrderTable',
        'MANUAL_INSERT',
        1,
        'admin',
        '2024-01-25 10:00:00',
        NULL,
        '{"OrderID": 1, "CustomerID": 1, "OrderStatus": "Pending"}' :: jsonb
    ),
    (
        'OrderItemTable',
        'MANUAL_INSERT',
        1,
        'admin',
        '2024-01-25 10:05:00',
        NULL,
        '{"ItemID": 1, "OrderID": 1, "ProductName": "Laptop Computer"}' :: jsonb
    ),
    (
        'CustomerTable',
        'MANUAL_UPDATE',
        2,
        'admin',
        '2024-01-16 15:00:00',
        '{"CustomerName": "Jane Doe", "Email": "jane.doe@test.com"}' :: jsonb,
        '{"CustomerName": "Jane Doe-Smith", "Email": "jane.doe@test.com"}' :: jsonb
    );


--conversion not supported
CREATE CONVERSION myconv FOR 'UTF8' TO 'LATIN1' FROM myfunc;

ALTER CONVERSION myconv rename to my_conv_1;

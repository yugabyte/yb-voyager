drop table if exists view_table1;

create table view_table1 (
	id int primary key comment 'this is the id',
	first_name VARCHAR(50),
	last_name VARCHAR(50),
	email VARCHAR(50),
	gender VARCHAR(50),
	ip_address VARCHAR(20)
)comment 'comment for the table';

delimiter //
Create Trigger before_insert_view_tab
before insert on view_table1 
FOR EACH ROW  
BEGIN  
IF NEW.id >5 then set new.id=new.id+1;
end if;
end
//
delimiter ;


insert into view_table1 (id,first_name, last_name, email, gender, ip_address) values (1,'Modestine', 'MacMeeking', 'mmacmeeking0@zimbio.com', 'Female', '208.44.58.185');
insert into view_table1 (id,first_name, last_name, email, gender, ip_address) values (2,'Genna', 'Kaysor', 'gkaysor1@hibu.com', 'Male', '202.48.51.58');
insert into view_table1 (id,first_name, last_name, email, gender, ip_address) values (3,'Tess', 'Wesker', 'twesker2@scientificamerican.com', 'Female', '177.153.32.186');
insert into view_table1 (id,first_name, last_name, email, gender, ip_address) values (4,'Magnum', 'Danzelman', 'mdanzelman3@storify.com', 'Bigender', '192.200.33.56');
insert into view_table1 (id,first_name, last_name, email, gender, ip_address) values (5,'Mitzi', 'Pidwell', 'mpidwell4@shutterfly.com', 'Female', '216.4.250.71');
insert into view_table1 (id,first_name, last_name, email, gender, ip_address) values (6,'Milzie', 'Rohlfing', 'mrohlfing5@java.com', 'Female', '230.101.87.42');

select * from view_table1;


drop procedure if exists insert_data;

delimiter //
CREATE PROCEDURE insert_data()
BEGIN
insert into view_table1 (id,first_name, last_name, email, gender, ip_address) values (7,'Milzie', 'Rohlfing', 'mrohlfing5@java.com', 'Female', '230.101.87.42');
END
//

delimiter ;




CREATE OPERATOR <% (
   leftarg = point,
   rightarg = base_type_examples.int42,
   procedure = base_type_examples.fake_op,
   commutator = >% ,
   negator = >=%
);

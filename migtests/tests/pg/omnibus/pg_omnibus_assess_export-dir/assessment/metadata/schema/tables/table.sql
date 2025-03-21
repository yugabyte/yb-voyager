-- setting variables for current session
SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;


CREATE TABLE fn_examples.ordinary_table (
    id integer
);


CREATE TABLE agg_ex.my_table (
    i integer
);


CREATE TABLE am_examples.am_partitioned (
    x integer,
    y integer
)
PARTITION BY HASH (x);


CREATE TABLE am_examples.fast_emp4000 (
    home_base box
);


CREATE TABLE am_examples.heaptable (
    a integer,
    repeat text
);


CREATE TABLE am_examples.tableam_parted_heapx (
    a text,
    b integer
)
PARTITION BY LIST (a);


CREATE TABLE am_examples.tableam_parted_1_heapx (
    a text,
    b integer
);


CREATE TABLE am_examples.tableam_parted_2_heapx (
    a text,
    b integer
);


CREATE TABLE am_examples.tableam_parted_heap2 (
    a text,
    b integer
)
PARTITION BY LIST (a);


CREATE TABLE am_examples.tableam_parted_c_heap2 (
    a text,
    b integer
);


CREATE TABLE am_examples.tableam_parted_d_heap2 (
    a text,
    b integer
);


CREATE TABLE am_examples.tableam_tbl_heap2 (
    f1 integer
);


CREATE TABLE am_examples.tableam_tbl_heapx (
    f1 integer
);


CREATE TABLE am_examples.tableam_tblas_heap2 (
    f1 integer
);


CREATE TABLE am_examples.tableam_tblas_heapx (
    f1 integer
);


CREATE TABLE base_type_examples.default_test (
    f1 base_type_examples.text_w_default,
    f2 base_type_examples.int42
);


CREATE TABLE composite_type_examples.ordinary_table (
    basic_ composite_type_examples.basic_comp_type,
    _basic composite_type_examples.basic_comp_type GENERATED ALWAYS AS (basic_) STORED,
    nested composite_type_examples.nested,
    _nested composite_type_examples.nested GENERATED ALWAYS AS (nested) STORED,
    CONSTRAINT check_f1_gt_1 CHECK (((basic_).f1 > 1)),
    CONSTRAINT check_f1_gt_1_again CHECK (((_basic).f1 > 1)),
    CONSTRAINT check_nested_f1_gt_1 CHECK (((nested).foo.f1 > 1)),
    CONSTRAINT check_nested_f1_gt_1_again CHECK (((_nested).foo.f1 > 1))
);


CREATE TABLE composite_type_examples.equivalent_rowtype (
    f1 integer,
    f2 text
);


CREATE TABLE composite_type_examples.i_0 (
    i integer
);


CREATE TABLE composite_type_examples.i_1 (
    i composite_type_examples.i_0
);


CREATE TABLE composite_type_examples.i_2 (
    i composite_type_examples.i_1
);


CREATE TABLE composite_type_examples.i_3 (
    i composite_type_examples.i_2
);


CREATE TABLE composite_type_examples.i_4 (
    i composite_type_examples.i_3
);


CREATE TABLE composite_type_examples.i_5 (
    i composite_type_examples.i_4
);


CREATE TABLE composite_type_examples.i_6 (
    i composite_type_examples.i_5
);


CREATE TABLE composite_type_examples.i_7 (
    i composite_type_examples.i_6
);


CREATE TABLE composite_type_examples.i_8 (
    i composite_type_examples.i_7
);


CREATE TABLE composite_type_examples.i_9 (
    i composite_type_examples.i_8
);


CREATE TABLE composite_type_examples.i_10 (
    i composite_type_examples.i_9
);


CREATE TABLE composite_type_examples.i_11 (
    i composite_type_examples.i_10
);


CREATE TABLE composite_type_examples.i_12 (
    i composite_type_examples.i_11
);


CREATE TABLE composite_type_examples.i_13 (
    i composite_type_examples.i_12
);


CREATE TABLE composite_type_examples.i_14 (
    i composite_type_examples.i_13
);


CREATE TABLE composite_type_examples.i_15 (
    i composite_type_examples.i_14
);


CREATE TABLE composite_type_examples.i_16 (
    i composite_type_examples.i_15
);


CREATE TABLE composite_type_examples.i_17 (
    i composite_type_examples.i_16
);


CREATE TABLE composite_type_examples.i_18 (
    i composite_type_examples.i_17
);


CREATE TABLE composite_type_examples.i_19 (
    i composite_type_examples.i_18
);


CREATE TABLE composite_type_examples.i_20 (
    i composite_type_examples.i_19
);


CREATE TABLE composite_type_examples.i_21 (
    i composite_type_examples.i_20
);


CREATE TABLE composite_type_examples.i_22 (
    i composite_type_examples.i_21
);


CREATE TABLE composite_type_examples.i_23 (
    i composite_type_examples.i_22
);


CREATE TABLE composite_type_examples.i_24 (
    i composite_type_examples.i_23
);


CREATE TABLE composite_type_examples.i_25 (
    i composite_type_examples.i_24
);


CREATE TABLE composite_type_examples.i_26 (
    i composite_type_examples.i_25
);


CREATE TABLE composite_type_examples.i_27 (
    i composite_type_examples.i_26
);


CREATE TABLE composite_type_examples.i_28 (
    i composite_type_examples.i_27
);


CREATE TABLE composite_type_examples.i_29 (
    i composite_type_examples.i_28
);


CREATE TABLE composite_type_examples.i_30 (
    i composite_type_examples.i_29
);


CREATE TABLE composite_type_examples.i_31 (
    i composite_type_examples.i_30
);


CREATE TABLE composite_type_examples.i_32 (
    i composite_type_examples.i_31
);


CREATE TABLE composite_type_examples.i_33 (
    i composite_type_examples.i_32
);


CREATE TABLE composite_type_examples.i_34 (
    i composite_type_examples.i_33
);


CREATE TABLE composite_type_examples.i_35 (
    i composite_type_examples.i_34
);


CREATE TABLE composite_type_examples.i_36 (
    i composite_type_examples.i_35
);


CREATE TABLE composite_type_examples.i_37 (
    i composite_type_examples.i_36
);


CREATE TABLE composite_type_examples.i_38 (
    i composite_type_examples.i_37
);


CREATE TABLE composite_type_examples.i_39 (
    i composite_type_examples.i_38
);


CREATE TABLE composite_type_examples.i_40 (
    i composite_type_examples.i_39
);


CREATE TABLE composite_type_examples.i_41 (
    i composite_type_examples.i_40
);


CREATE TABLE composite_type_examples.i_42 (
    i composite_type_examples.i_41
);


CREATE TABLE composite_type_examples.i_43 (
    i composite_type_examples.i_42
);


CREATE TABLE composite_type_examples.i_44 (
    i composite_type_examples.i_43
);


CREATE TABLE composite_type_examples.i_45 (
    i composite_type_examples.i_44
);


CREATE TABLE composite_type_examples.i_46 (
    i composite_type_examples.i_45
);


CREATE TABLE composite_type_examples.i_47 (
    i composite_type_examples.i_46
);


CREATE TABLE composite_type_examples.i_48 (
    i composite_type_examples.i_47
);


CREATE TABLE composite_type_examples.i_49 (
    i composite_type_examples.i_48
);


CREATE TABLE composite_type_examples.i_50 (
    i composite_type_examples.i_49
);


CREATE TABLE composite_type_examples.i_51 (
    i composite_type_examples.i_50
);


CREATE TABLE composite_type_examples.i_52 (
    i composite_type_examples.i_51
);


CREATE TABLE composite_type_examples.i_53 (
    i composite_type_examples.i_52
);


CREATE TABLE composite_type_examples.i_54 (
    i composite_type_examples.i_53
);


CREATE TABLE composite_type_examples.i_55 (
    i composite_type_examples.i_54
);


CREATE TABLE composite_type_examples.i_56 (
    i composite_type_examples.i_55
);


CREATE TABLE composite_type_examples.i_57 (
    i composite_type_examples.i_56
);


CREATE TABLE composite_type_examples.i_58 (
    i composite_type_examples.i_57
);


CREATE TABLE composite_type_examples.i_59 (
    i composite_type_examples.i_58
);


CREATE TABLE composite_type_examples.i_60 (
    i composite_type_examples.i_59
);


CREATE TABLE composite_type_examples.i_61 (
    i composite_type_examples.i_60
);


CREATE TABLE composite_type_examples.i_62 (
    i composite_type_examples.i_61
);


CREATE TABLE composite_type_examples.i_63 (
    i composite_type_examples.i_62
);


CREATE TABLE composite_type_examples.i_64 (
    i composite_type_examples.i_63
);


CREATE TABLE composite_type_examples.i_65 (
    i composite_type_examples.i_64
);


CREATE TABLE composite_type_examples.i_66 (
    i composite_type_examples.i_65
);


CREATE TABLE composite_type_examples.i_67 (
    i composite_type_examples.i_66
);


CREATE TABLE composite_type_examples.i_68 (
    i composite_type_examples.i_67
);


CREATE TABLE composite_type_examples.i_69 (
    i composite_type_examples.i_68
);


CREATE TABLE composite_type_examples.i_70 (
    i composite_type_examples.i_69
);


CREATE TABLE composite_type_examples.i_71 (
    i composite_type_examples.i_70
);


CREATE TABLE composite_type_examples.i_72 (
    i composite_type_examples.i_71
);


CREATE TABLE composite_type_examples.i_73 (
    i composite_type_examples.i_72
);


CREATE TABLE composite_type_examples.i_74 (
    i composite_type_examples.i_73
);


CREATE TABLE composite_type_examples.i_75 (
    i composite_type_examples.i_74
);


CREATE TABLE composite_type_examples.i_76 (
    i composite_type_examples.i_75
);


CREATE TABLE composite_type_examples.i_77 (
    i composite_type_examples.i_76
);


CREATE TABLE composite_type_examples.i_78 (
    i composite_type_examples.i_77
);


CREATE TABLE composite_type_examples.i_79 (
    i composite_type_examples.i_78
);


CREATE TABLE composite_type_examples.i_80 (
    i composite_type_examples.i_79
);


CREATE TABLE composite_type_examples.i_81 (
    i composite_type_examples.i_80
);


CREATE TABLE composite_type_examples.i_82 (
    i composite_type_examples.i_81
);


CREATE TABLE composite_type_examples.i_83 (
    i composite_type_examples.i_82
);


CREATE TABLE composite_type_examples.i_84 (
    i composite_type_examples.i_83
);


CREATE TABLE composite_type_examples.i_85 (
    i composite_type_examples.i_84
);


CREATE TABLE composite_type_examples.i_86 (
    i composite_type_examples.i_85
);


CREATE TABLE composite_type_examples.i_87 (
    i composite_type_examples.i_86
);


CREATE TABLE composite_type_examples.i_88 (
    i composite_type_examples.i_87
);


CREATE TABLE composite_type_examples.i_89 (
    i composite_type_examples.i_88
);


CREATE TABLE composite_type_examples.i_90 (
    i composite_type_examples.i_89
);


CREATE TABLE composite_type_examples.i_91 (
    i composite_type_examples.i_90
);


CREATE TABLE composite_type_examples.i_92 (
    i composite_type_examples.i_91
);


CREATE TABLE composite_type_examples.i_93 (
    i composite_type_examples.i_92
);


CREATE TABLE composite_type_examples.i_94 (
    i composite_type_examples.i_93
);


CREATE TABLE composite_type_examples.i_95 (
    i composite_type_examples.i_94
);


CREATE TABLE composite_type_examples.i_96 (
    i composite_type_examples.i_95
);


CREATE TABLE composite_type_examples.i_97 (
    i composite_type_examples.i_96
);


CREATE TABLE composite_type_examples.i_98 (
    i composite_type_examples.i_97
);


CREATE TABLE composite_type_examples.i_99 (
    i composite_type_examples.i_98
);


CREATE TABLE composite_type_examples.i_100 (
    i composite_type_examples.i_99
);


CREATE TABLE composite_type_examples.i_101 (
    i composite_type_examples.i_100
);


CREATE TABLE composite_type_examples.i_102 (
    i composite_type_examples.i_101
);


CREATE TABLE composite_type_examples.i_103 (
    i composite_type_examples.i_102
);


CREATE TABLE composite_type_examples.i_104 (
    i composite_type_examples.i_103
);


CREATE TABLE composite_type_examples.i_105 (
    i composite_type_examples.i_104
);


CREATE TABLE composite_type_examples.i_106 (
    i composite_type_examples.i_105
);


CREATE TABLE composite_type_examples.i_107 (
    i composite_type_examples.i_106
);


CREATE TABLE composite_type_examples.i_108 (
    i composite_type_examples.i_107
);


CREATE TABLE composite_type_examples.i_109 (
    i composite_type_examples.i_108
);


CREATE TABLE composite_type_examples.i_110 (
    i composite_type_examples.i_109
);


CREATE TABLE composite_type_examples.i_111 (
    i composite_type_examples.i_110
);


CREATE TABLE composite_type_examples.i_112 (
    i composite_type_examples.i_111
);


CREATE TABLE composite_type_examples.i_113 (
    i composite_type_examples.i_112
);


CREATE TABLE composite_type_examples.i_114 (
    i composite_type_examples.i_113
);


CREATE TABLE composite_type_examples.i_115 (
    i composite_type_examples.i_114
);


CREATE TABLE composite_type_examples.i_116 (
    i composite_type_examples.i_115
);


CREATE TABLE composite_type_examples.i_117 (
    i composite_type_examples.i_116
);


CREATE TABLE composite_type_examples.i_118 (
    i composite_type_examples.i_117
);


CREATE TABLE composite_type_examples.i_119 (
    i composite_type_examples.i_118
);


CREATE TABLE composite_type_examples.i_120 (
    i composite_type_examples.i_119
);


CREATE TABLE composite_type_examples.i_121 (
    i composite_type_examples.i_120
);


CREATE TABLE composite_type_examples.i_122 (
    i composite_type_examples.i_121
);


CREATE TABLE composite_type_examples.i_123 (
    i composite_type_examples.i_122
);


CREATE TABLE composite_type_examples.i_124 (
    i composite_type_examples.i_123
);


CREATE TABLE composite_type_examples.i_125 (
    i composite_type_examples.i_124
);


CREATE TABLE composite_type_examples.i_126 (
    i composite_type_examples.i_125
);


CREATE TABLE composite_type_examples.i_127 (
    i composite_type_examples.i_126
);


CREATE TABLE composite_type_examples.i_128 (
    i composite_type_examples.i_127
);


CREATE TABLE composite_type_examples.i_129 (
    i composite_type_examples.i_128
);


CREATE TABLE composite_type_examples.i_130 (
    i composite_type_examples.i_129
);


CREATE TABLE composite_type_examples.i_131 (
    i composite_type_examples.i_130
);


CREATE TABLE composite_type_examples.i_132 (
    i composite_type_examples.i_131
);


CREATE TABLE composite_type_examples.i_133 (
    i composite_type_examples.i_132
);


CREATE TABLE composite_type_examples.i_134 (
    i composite_type_examples.i_133
);


CREATE TABLE composite_type_examples.i_135 (
    i composite_type_examples.i_134
);


CREATE TABLE composite_type_examples.i_136 (
    i composite_type_examples.i_135
);


CREATE TABLE composite_type_examples.i_137 (
    i composite_type_examples.i_136
);


CREATE TABLE composite_type_examples.i_138 (
    i composite_type_examples.i_137
);


CREATE TABLE composite_type_examples.i_139 (
    i composite_type_examples.i_138
);


CREATE TABLE composite_type_examples.i_140 (
    i composite_type_examples.i_139
);


CREATE TABLE composite_type_examples.i_141 (
    i composite_type_examples.i_140
);


CREATE TABLE composite_type_examples.i_142 (
    i composite_type_examples.i_141
);


CREATE TABLE composite_type_examples.i_143 (
    i composite_type_examples.i_142
);


CREATE TABLE composite_type_examples.i_144 (
    i composite_type_examples.i_143
);


CREATE TABLE composite_type_examples.i_145 (
    i composite_type_examples.i_144
);


CREATE TABLE composite_type_examples.i_146 (
    i composite_type_examples.i_145
);


CREATE TABLE composite_type_examples.i_147 (
    i composite_type_examples.i_146
);


CREATE TABLE composite_type_examples.i_148 (
    i composite_type_examples.i_147
);


CREATE TABLE composite_type_examples.i_149 (
    i composite_type_examples.i_148
);


CREATE TABLE composite_type_examples.i_150 (
    i composite_type_examples.i_149
);


CREATE TABLE composite_type_examples.i_151 (
    i composite_type_examples.i_150
);


CREATE TABLE composite_type_examples.i_152 (
    i composite_type_examples.i_151
);


CREATE TABLE composite_type_examples.i_153 (
    i composite_type_examples.i_152
);


CREATE TABLE composite_type_examples.i_154 (
    i composite_type_examples.i_153
);


CREATE TABLE composite_type_examples.i_155 (
    i composite_type_examples.i_154
);


CREATE TABLE composite_type_examples.i_156 (
    i composite_type_examples.i_155
);


CREATE TABLE composite_type_examples.i_157 (
    i composite_type_examples.i_156
);


CREATE TABLE composite_type_examples.i_158 (
    i composite_type_examples.i_157
);


CREATE TABLE composite_type_examples.i_159 (
    i composite_type_examples.i_158
);


CREATE TABLE composite_type_examples.i_160 (
    i composite_type_examples.i_159
);


CREATE TABLE composite_type_examples.i_161 (
    i composite_type_examples.i_160
);


CREATE TABLE composite_type_examples.i_162 (
    i composite_type_examples.i_161
);


CREATE TABLE composite_type_examples.i_163 (
    i composite_type_examples.i_162
);


CREATE TABLE composite_type_examples.i_164 (
    i composite_type_examples.i_163
);


CREATE TABLE composite_type_examples.i_165 (
    i composite_type_examples.i_164
);


CREATE TABLE composite_type_examples.i_166 (
    i composite_type_examples.i_165
);


CREATE TABLE composite_type_examples.i_167 (
    i composite_type_examples.i_166
);


CREATE TABLE composite_type_examples.i_168 (
    i composite_type_examples.i_167
);


CREATE TABLE composite_type_examples.i_169 (
    i composite_type_examples.i_168
);


CREATE TABLE composite_type_examples.i_170 (
    i composite_type_examples.i_169
);


CREATE TABLE composite_type_examples.i_171 (
    i composite_type_examples.i_170
);


CREATE TABLE composite_type_examples.i_172 (
    i composite_type_examples.i_171
);


CREATE TABLE composite_type_examples.i_173 (
    i composite_type_examples.i_172
);


CREATE TABLE composite_type_examples.i_174 (
    i composite_type_examples.i_173
);


CREATE TABLE composite_type_examples.i_175 (
    i composite_type_examples.i_174
);


CREATE TABLE composite_type_examples.i_176 (
    i composite_type_examples.i_175
);


CREATE TABLE composite_type_examples.i_177 (
    i composite_type_examples.i_176
);


CREATE TABLE composite_type_examples.i_178 (
    i composite_type_examples.i_177
);


CREATE TABLE composite_type_examples.i_179 (
    i composite_type_examples.i_178
);


CREATE TABLE composite_type_examples.i_180 (
    i composite_type_examples.i_179
);


CREATE TABLE composite_type_examples.i_181 (
    i composite_type_examples.i_180
);


CREATE TABLE composite_type_examples.i_182 (
    i composite_type_examples.i_181
);


CREATE TABLE composite_type_examples.i_183 (
    i composite_type_examples.i_182
);


CREATE TABLE composite_type_examples.i_184 (
    i composite_type_examples.i_183
);


CREATE TABLE composite_type_examples.i_185 (
    i composite_type_examples.i_184
);


CREATE TABLE composite_type_examples.i_186 (
    i composite_type_examples.i_185
);


CREATE TABLE composite_type_examples.i_187 (
    i composite_type_examples.i_186
);


CREATE TABLE composite_type_examples.i_188 (
    i composite_type_examples.i_187
);


CREATE TABLE composite_type_examples.i_189 (
    i composite_type_examples.i_188
);


CREATE TABLE composite_type_examples.i_190 (
    i composite_type_examples.i_189
);


CREATE TABLE composite_type_examples.i_191 (
    i composite_type_examples.i_190
);


CREATE TABLE composite_type_examples.i_192 (
    i composite_type_examples.i_191
);


CREATE TABLE composite_type_examples.i_193 (
    i composite_type_examples.i_192
);


CREATE TABLE composite_type_examples.i_194 (
    i composite_type_examples.i_193
);


CREATE TABLE composite_type_examples.i_195 (
    i composite_type_examples.i_194
);


CREATE TABLE composite_type_examples.i_196 (
    i composite_type_examples.i_195
);


CREATE TABLE composite_type_examples.i_197 (
    i composite_type_examples.i_196
);


CREATE TABLE composite_type_examples.i_198 (
    i composite_type_examples.i_197
);


CREATE TABLE composite_type_examples.i_199 (
    i composite_type_examples.i_198
);


CREATE TABLE composite_type_examples.i_200 (
    i composite_type_examples.i_199
);


CREATE TABLE composite_type_examples.i_201 (
    i composite_type_examples.i_200
);


CREATE TABLE composite_type_examples.i_202 (
    i composite_type_examples.i_201
);


CREATE TABLE composite_type_examples.i_203 (
    i composite_type_examples.i_202
);


CREATE TABLE composite_type_examples.i_204 (
    i composite_type_examples.i_203
);


CREATE TABLE composite_type_examples.i_205 (
    i composite_type_examples.i_204
);


CREATE TABLE composite_type_examples.i_206 (
    i composite_type_examples.i_205
);


CREATE TABLE composite_type_examples.i_207 (
    i composite_type_examples.i_206
);


CREATE TABLE composite_type_examples.i_208 (
    i composite_type_examples.i_207
);


CREATE TABLE composite_type_examples.i_209 (
    i composite_type_examples.i_208
);


CREATE TABLE composite_type_examples.i_210 (
    i composite_type_examples.i_209
);


CREATE TABLE composite_type_examples.i_211 (
    i composite_type_examples.i_210
);


CREATE TABLE composite_type_examples.i_212 (
    i composite_type_examples.i_211
);


CREATE TABLE composite_type_examples.i_213 (
    i composite_type_examples.i_212
);


CREATE TABLE composite_type_examples.i_214 (
    i composite_type_examples.i_213
);


CREATE TABLE composite_type_examples.i_215 (
    i composite_type_examples.i_214
);


CREATE TABLE composite_type_examples.i_216 (
    i composite_type_examples.i_215
);


CREATE TABLE composite_type_examples.i_217 (
    i composite_type_examples.i_216
);


CREATE TABLE composite_type_examples.i_218 (
    i composite_type_examples.i_217
);


CREATE TABLE composite_type_examples.i_219 (
    i composite_type_examples.i_218
);


CREATE TABLE composite_type_examples.i_220 (
    i composite_type_examples.i_219
);


CREATE TABLE composite_type_examples.i_221 (
    i composite_type_examples.i_220
);


CREATE TABLE composite_type_examples.i_222 (
    i composite_type_examples.i_221
);


CREATE TABLE composite_type_examples.i_223 (
    i composite_type_examples.i_222
);


CREATE TABLE composite_type_examples.i_224 (
    i composite_type_examples.i_223
);


CREATE TABLE composite_type_examples.i_225 (
    i composite_type_examples.i_224
);


CREATE TABLE composite_type_examples.i_226 (
    i composite_type_examples.i_225
);


CREATE TABLE composite_type_examples.i_227 (
    i composite_type_examples.i_226
);


CREATE TABLE composite_type_examples.i_228 (
    i composite_type_examples.i_227
);


CREATE TABLE composite_type_examples.i_229 (
    i composite_type_examples.i_228
);


CREATE TABLE composite_type_examples.i_230 (
    i composite_type_examples.i_229
);


CREATE TABLE composite_type_examples.i_231 (
    i composite_type_examples.i_230
);


CREATE TABLE composite_type_examples.i_232 (
    i composite_type_examples.i_231
);


CREATE TABLE composite_type_examples.i_233 (
    i composite_type_examples.i_232
);


CREATE TABLE composite_type_examples.i_234 (
    i composite_type_examples.i_233
);


CREATE TABLE composite_type_examples.i_235 (
    i composite_type_examples.i_234
);


CREATE TABLE composite_type_examples.i_236 (
    i composite_type_examples.i_235
);


CREATE TABLE composite_type_examples.i_237 (
    i composite_type_examples.i_236
);


CREATE TABLE composite_type_examples.i_238 (
    i composite_type_examples.i_237
);


CREATE TABLE composite_type_examples.i_239 (
    i composite_type_examples.i_238
);


CREATE TABLE composite_type_examples.i_240 (
    i composite_type_examples.i_239
);


CREATE TABLE composite_type_examples.i_241 (
    i composite_type_examples.i_240
);


CREATE TABLE composite_type_examples.i_242 (
    i composite_type_examples.i_241
);


CREATE TABLE composite_type_examples.i_243 (
    i composite_type_examples.i_242
);


CREATE TABLE composite_type_examples.i_244 (
    i composite_type_examples.i_243
);


CREATE TABLE composite_type_examples.i_245 (
    i composite_type_examples.i_244
);


CREATE TABLE composite_type_examples.i_246 (
    i composite_type_examples.i_245
);


CREATE TABLE composite_type_examples.i_247 (
    i composite_type_examples.i_246
);


CREATE TABLE composite_type_examples.i_248 (
    i composite_type_examples.i_247
);


CREATE TABLE composite_type_examples.i_249 (
    i composite_type_examples.i_248
);


CREATE TABLE composite_type_examples.i_250 (
    i composite_type_examples.i_249
);


CREATE TABLE composite_type_examples.i_251 (
    i composite_type_examples.i_250
);


CREATE TABLE composite_type_examples.i_252 (
    i composite_type_examples.i_251
);


CREATE TABLE composite_type_examples.i_253 (
    i composite_type_examples.i_252
);


CREATE TABLE composite_type_examples.i_254 (
    i composite_type_examples.i_253
);


CREATE TABLE composite_type_examples.i_255 (
    i composite_type_examples.i_254
);


CREATE TABLE composite_type_examples.i_256 (
    i composite_type_examples.i_255
);


CREATE TABLE composite_type_examples.inherited_table (
)
INHERITS (composite_type_examples.ordinary_table);


CREATE TABLE domain_examples.even_numbers (
    e domain_examples.positive_even_number
);


CREATE TABLE domain_examples.us_snail_addy (
    address_id integer NOT NULL,
    street1 text NOT NULL,
    street2 text,
    street3 text,
    city text NOT NULL,
    postal domain_examples.us_postal_code NOT NULL
);


CREATE TABLE enum_example._bug_severity (
    id integer,
    severity enum_example.bug_severity
);


CREATE TABLE enum_example.bugs (
    id integer NOT NULL,
    description text,
    status enum_example.bug_status,
    _status enum_example.bug_status GENERATED ALWAYS AS (status) STORED,
    severity enum_example.bug_severity,
    _severity enum_example.bug_severity GENERATED ALWAYS AS (severity) STORED,
    info enum_example.bug_info GENERATED ALWAYS AS (enum_example.make_bug_info(status, severity)) STORED
);


CREATE TABLE enum_example.bugs_clone (
)
INHERITS (enum_example.bugs);


CREATE TABLE extension_example.testhstore (
    h extension_example.hstore
);


CREATE TABLE idx_ex.films (
    id integer NOT NULL,
    title text,
    director text,
    rating integer,
    code text
);


CREATE TABLE ordinary_tables.binary_examples (
    bytes bytea NOT NULL
);


CREATE TABLE ordinary_tables.bit_string_examples (
    bit_example bit(10),
    bit_varyint_example bit varying(20)
);


CREATE TABLE ordinary_tables.boolean_examples (
    b boolean
);


CREATE TABLE ordinary_tables.character_examples (
    id text NOT NULL,
    a_varchar character varying,
    a_limited_varchar character varying(10),
    a_single_char character(1),
    n_char character(11)
);


CREATE TABLE ordinary_tables.geometric_examples (
    point_example point,
    line_example line,
    lseg_example lseg,
    box_example box,
    path_example path,
    polygon_example polygon,
    circle_example circle
);


CREATE TABLE ordinary_tables.money_example (
    money money
);


CREATE TABLE ordinary_tables.network_addr_examples (
    cidr_example cidr,
    inet_example inet,
    macaddr_example macaddr,
    macaddr8_example macaddr8
);


CREATE TABLE ordinary_tables.numeric_type_examples (
    id integer NOT NULL,
    an_integer integer NOT NULL,
    an_int integer,
    an_int4 integer,
    an_int8 bigint,
    a_bigint bigint,
    a_smallint smallint,
    a_decimal numeric,
    a_numeric numeric,
    a_real real,
    a_double double precision,
    a_smallserial smallint NOT NULL,
    a_bigserial bigint NOT NULL,
    another_numeric numeric(3,0),
    yet_another_numeric numeric(6,4)
);


CREATE TABLE ordinary_tables."time" (
    ts_with_tz timestamp with time zone,
    ts_with_tz_precision timestamp(2) with time zone,
    ts_with_ntz timestamp without time zone,
    ts_with_ntz_precision timestamp(3) without time zone,
    t_with_tz time with time zone,
    t_with_tz_precision time(4) with time zone,
    t_with_ntz time without time zone,
    t_with_ntz_precision time(5) without time zone,
    date date,
    interval_year interval year,
    interval_month interval month,
    interval_day interval day,
    interval_hour interval hour,
    interval_minute interval minute,
    interval_second interval second,
    interval_year_to_month interval year to month,
    interval_day_to_hour interval day to hour,
    interval_day_to_minute interval day to minute,
    interval_day_to_second interval day to second,
    interval_hour_to_minute interval hour to minute,
    interval_hour_to_second interval hour to second,
    interval_minute_to_second interval minute to second
);


CREATE TABLE range_type_example.example_tbl (
    col range_type_example.float8_range
);


CREATE TABLE regress_rls_schema.b1 (
    a integer,
    b text
);


CREATE TABLE regress_rls_schema.category (
    cid integer NOT NULL,
    cname text
);


CREATE TABLE regress_rls_schema.dependee (
    x integer,
    y integer
);


CREATE TABLE regress_rls_schema.dependent (
    x integer,
    y integer
);


CREATE TABLE regress_rls_schema.dob_t1 (
    c1 integer
);


CREATE TABLE regress_rls_schema.dob_t2 (
    c1 integer
)
PARTITION BY RANGE (c1);


CREATE TABLE regress_rls_schema.document (
    did integer NOT NULL,
    cid integer,
    dlevel integer NOT NULL,
    dauthor name,
    dtitle text,
    dnotes text DEFAULT ''::text
);


CREATE TABLE regress_rls_schema.part_document (
    did integer,
    cid integer,
    dlevel integer NOT NULL,
    dauthor name,
    dtitle text
)
PARTITION BY RANGE (cid);


CREATE TABLE regress_rls_schema.part_document_fiction (
    did integer,
    cid integer,
    dlevel integer NOT NULL,
    dauthor name,
    dtitle text
);


CREATE TABLE regress_rls_schema.part_document_nonfiction (
    did integer,
    cid integer,
    dlevel integer NOT NULL,
    dauthor name,
    dtitle text
);


CREATE TABLE regress_rls_schema.part_document_satire (
    did integer,
    cid integer,
    dlevel integer NOT NULL,
    dauthor name,
    dtitle text
);


CREATE TABLE regress_rls_schema.r1 (
    a integer NOT NULL
);


CREATE TABLE regress_rls_schema.r1_2 (
    a integer
);
ALTER TABLE ONLY regress_rls_schema.r1_2 FORCE ROW LEVEL SECURITY;


CREATE TABLE regress_rls_schema.r1_3 (
    a integer NOT NULL
);
ALTER TABLE ONLY regress_rls_schema.r1_3 FORCE ROW LEVEL SECURITY;


CREATE TABLE regress_rls_schema.r1_4 (
    a integer NOT NULL
);


CREATE TABLE regress_rls_schema.r1_5 (
    a integer NOT NULL
);


CREATE TABLE regress_rls_schema.r2 (
    a integer
);


CREATE TABLE regress_rls_schema.r2_3 (
    a integer
);
ALTER TABLE ONLY regress_rls_schema.r2_3 FORCE ROW LEVEL SECURITY;


CREATE TABLE regress_rls_schema.r2_4 (
    a integer
);
ALTER TABLE ONLY regress_rls_schema.r2_4 FORCE ROW LEVEL SECURITY;


CREATE TABLE regress_rls_schema.r2_5 (
    a integer
);
ALTER TABLE ONLY regress_rls_schema.r2_5 FORCE ROW LEVEL SECURITY;


CREATE TABLE regress_rls_schema.rec1 (
    x integer,
    y integer
);


CREATE TABLE regress_rls_schema.rec2 (
    a integer,
    b integer
);


CREATE TABLE regress_rls_schema.ref_tbl (
    a integer
);


CREATE TABLE regress_rls_schema.y1 (
    a integer,
    b text
);


CREATE TABLE regress_rls_schema.rls_tbl (
    a integer
);


CREATE TABLE regress_rls_schema.rls_tbl_2 (
    a integer
);


CREATE TABLE regress_rls_schema.rls_tbl_3 (
    a integer,
    b integer,
    c integer
);
ALTER TABLE ONLY regress_rls_schema.rls_tbl_3 FORCE ROW LEVEL SECURITY;


CREATE TABLE regress_rls_schema.z1 (
    a integer,
    b text
);


CREATE TABLE regress_rls_schema.s1 (
    a integer,
    b text
);


CREATE TABLE regress_rls_schema.s2 (
    x integer,
    y text
);


CREATE TABLE regress_rls_schema.t (
    c integer
);


CREATE TABLE regress_rls_schema.t1 (
    id integer NOT NULL,
    a integer,
    b text
);


CREATE TABLE regress_rls_schema.t1_2 (
    a integer
);


CREATE TABLE regress_rls_schema.t1_3 (
    a integer,
    b text
);


CREATE TABLE regress_rls_schema.t2 (
    c double precision
)
INHERITS (regress_rls_schema.t1);


CREATE TABLE regress_rls_schema.t2_3 (
    a integer,
    b text
);


CREATE TABLE regress_rls_schema.t3_3 (
    id integer NOT NULL,
    c text,
    b text,
    a integer
)
INHERITS (regress_rls_schema.t1_3);


CREATE TABLE regress_rls_schema.tbl1 (
    c text
);


CREATE TABLE regress_rls_schema.test_qual_pushdown (
    abc text
);


CREATE TABLE regress_rls_schema.uaccount (
    pguser name NOT NULL,
    seclv integer
);


CREATE TABLE regress_rls_schema.x1 (
    a integer,
    b text,
    c text
);


CREATE TABLE regress_rls_schema.y2 (
    a integer,
    b text
);


CREATE TABLE regress_rls_schema.z1_blacklist (
    a integer
);


CREATE TABLE regress_rls_schema.z2 (
    a integer,
    b text
);


CREATE TABLE trigger_test.accounts (
    id integer NOT NULL,
    balance real
);


CREATE TABLE trigger_test.update_log (
    "timestamp" timestamp without time zone DEFAULT now() NOT NULL,
    account_id integer NOT NULL
);


ALTER TABLE ONLY domain_examples.us_snail_addy ALTER COLUMN address_id SET DEFAULT nextval('domain_examples.us_snail_addy_address_id_seq'::regclass);


ALTER TABLE ONLY enum_example.bugs ALTER COLUMN id SET DEFAULT nextval('enum_example.bugs_id_seq'::regclass);


ALTER TABLE ONLY enum_example.bugs_clone ALTER COLUMN id SET DEFAULT nextval('enum_example.bugs_id_seq'::regclass);


ALTER TABLE ONLY ordinary_tables.numeric_type_examples ALTER COLUMN id SET DEFAULT nextval('ordinary_tables.numeric_type_examples_id_seq'::regclass);


ALTER TABLE ONLY ordinary_tables.numeric_type_examples ALTER COLUMN a_smallserial SET DEFAULT nextval('ordinary_tables.numeric_type_examples_a_smallserial_seq'::regclass);


ALTER TABLE ONLY ordinary_tables.numeric_type_examples ALTER COLUMN a_bigserial SET DEFAULT nextval('ordinary_tables.numeric_type_examples_a_bigserial_seq'::regclass);


ALTER TABLE ONLY domain_examples.us_snail_addy
    ADD CONSTRAINT us_snail_addy_pkey PRIMARY KEY (address_id);


ALTER TABLE ONLY idx_ex.films
    ADD CONSTRAINT films_pkey PRIMARY KEY (id);


ALTER TABLE ONLY ordinary_tables.binary_examples
    ADD CONSTRAINT binary_examples_pkey PRIMARY KEY (bytes);


ALTER TABLE ONLY ordinary_tables.character_examples
    ADD CONSTRAINT character_examples_pkey PRIMARY KEY (id);


ALTER TABLE ONLY ordinary_tables.numeric_type_examples
    ADD CONSTRAINT numeric_type_examples_pkey PRIMARY KEY (id);


ALTER TABLE ONLY regress_rls_schema.category
    ADD CONSTRAINT category_pkey PRIMARY KEY (cid);


ALTER TABLE ONLY regress_rls_schema.document
    ADD CONSTRAINT document_pkey PRIMARY KEY (did);


ALTER TABLE ONLY regress_rls_schema.r1_3
    ADD CONSTRAINT r1_3_pkey PRIMARY KEY (a);


ALTER TABLE ONLY regress_rls_schema.r1_4
    ADD CONSTRAINT r1_4_pkey PRIMARY KEY (a);


ALTER TABLE ONLY regress_rls_schema.r1_5
    ADD CONSTRAINT r1_5_pkey PRIMARY KEY (a);


ALTER TABLE ONLY regress_rls_schema.r1
    ADD CONSTRAINT r1_pkey PRIMARY KEY (a);


ALTER TABLE ONLY regress_rls_schema.t1
    ADD CONSTRAINT t1_pkey PRIMARY KEY (id);


ALTER TABLE ONLY regress_rls_schema.t3_3
    ADD CONSTRAINT t3_3_pkey PRIMARY KEY (id);


ALTER TABLE ONLY regress_rls_schema.uaccount
    ADD CONSTRAINT uaccount_pkey PRIMARY KEY (pguser);


ALTER TABLE ONLY trigger_test.accounts
    ADD CONSTRAINT accounts_pkey PRIMARY KEY (id);


ALTER TABLE ONLY trigger_test.update_log
    ADD CONSTRAINT update_log_pk PRIMARY KEY ("timestamp", account_id);


ALTER TABLE ONLY regress_rls_schema.document
    ADD CONSTRAINT document_cid_fkey FOREIGN KEY (cid) REFERENCES regress_rls_schema.category(cid);


ALTER TABLE ONLY regress_rls_schema.r2_3
    ADD CONSTRAINT r2_3_a_fkey FOREIGN KEY (a) REFERENCES regress_rls_schema.r1(a);


ALTER TABLE ONLY regress_rls_schema.r2_4
    ADD CONSTRAINT r2_4_a_fkey FOREIGN KEY (a) REFERENCES regress_rls_schema.r1(a) ON DELETE CASCADE;


ALTER TABLE ONLY regress_rls_schema.r2_5
    ADD CONSTRAINT r2_5_a_fkey FOREIGN KEY (a) REFERENCES regress_rls_schema.r1(a) ON UPDATE CASCADE;


ALTER TABLE ONLY trigger_test.update_log
    ADD CONSTRAINT update_log_account_id_fkey FOREIGN KEY (account_id) REFERENCES trigger_test.accounts(id);


ALTER TABLE regress_rls_schema.b1 ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.document ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.part_document ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.r1 ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.r1_2 ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.r1_3 ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.r2 ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.r2_3 ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.r2_4 ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.r2_5 ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.rec1 ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.rec2 ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.rls_tbl ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.rls_tbl_2 ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.rls_tbl_3 ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.s1 ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.s2 ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.t ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.t1 ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.t1_2 ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.t1_3 ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.t2 ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.t2_3 ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.x1 ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.y1 ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.y2 ENABLE ROW LEVEL SECURITY;


ALTER TABLE regress_rls_schema.z1 ENABLE ROW LEVEL SECURITY;


ALTER TABLE ONLY am_examples.tableam_parted_heapx ATTACH PARTITION am_examples.tableam_parted_1_heapx FOR VALUES IN ('a', 'b');


ALTER TABLE ONLY am_examples.tableam_parted_heapx ATTACH PARTITION am_examples.tableam_parted_2_heapx FOR VALUES IN ('c', 'd');


ALTER TABLE ONLY am_examples.tableam_parted_heap2 ATTACH PARTITION am_examples.tableam_parted_c_heap2 FOR VALUES IN ('c');


ALTER TABLE ONLY am_examples.tableam_parted_heap2 ATTACH PARTITION am_examples.tableam_parted_d_heap2 FOR VALUES IN ('d');


ALTER TABLE ONLY regress_rls_schema.part_document ATTACH PARTITION regress_rls_schema.part_document_fiction FOR VALUES FROM (11) TO (12);


ALTER TABLE ONLY regress_rls_schema.part_document ATTACH PARTITION regress_rls_schema.part_document_nonfiction FOR VALUES FROM (99) TO (100);


ALTER TABLE ONLY regress_rls_schema.part_document ATTACH PARTITION regress_rls_schema.part_document_satire FOR VALUES FROM (55) TO (56);



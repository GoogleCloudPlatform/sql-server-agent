dbset db mssqls
dbset bm TPROC-C
diset connection mssqls_server %SERVER%
diset connection mssqls_tcp true
diset connection mssqls_port 1433
diset connection mssqls_authentication sql
diset connection mssqls_odbc_driver {ODBC Driver 17 for SQL Server}
diset connection mssqls_uid sa
diset connection mssqls_pass %PASSWORD%
diset connection mssqls_encrypt_connection true
diset connection mssqls_trust_server_cert true
diset tpcc mssqls_count_ware 40
diset tpcc mssqls_num_vu 8
diset tpcc mssqls_dbase tpcc
buildschema
diset tpcc mssqls_bucket 1
diset tpcc mssqls_durability SCHEMA_AND_DATA
diset tpcc mssqls_driver timed
diset tpcc mssqls_use_bcp true
diset tpcc mssqls_rampup 5
diset tpcc mssqls_duration 20
loadscript

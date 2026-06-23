-- Set the temp file limit to 5GB to avoid the error "ERROR: temp file limit exceeded" while row_hash_validations in tests
ALTER DATABASE test_db SET temp_file_limit = 5242880; -- 5GB
/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package queryparser

import (
	"strings"
)

const (
	PLPGSQL_EXPR = "PLpgSQL_expr"
	QUERY        = "query"
)

/*
Query example-

	CREATE OR REPLACE FUNCTION func_example_advi(
	   sender_id INT,
	   receiver_id INT,
	   transfer_amount NUMERIC

)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN

	-- Acquire advisory locks to prevent concurrent updates on the same accounts
	PERFORM pg_advisory_lock(sender_id);
	PERFORM pg_advisory_lock(receiver_id);

	-- Deduct the amount from the sender's account
	UPDATE accounts
	SET balance = balance - transfer_amount
	WHERE account_id = sender_id;

	-- Add the amount to the receiver's account
	UPDATE accounts
	SET balance = balance + transfer_amount
	WHERE account_id = receiver_id;

	-- Release the advisory locks
	PERFORM pg_advisory_unlock(sender_id);
	PERFORM pg_advisory_unlock(receiver_id);

END;
$$;

parsed json -
	{
	  "PLpgSQL_function": {
	    "datums": [
	      ...
	    ],
	    "action": {
	      "PLpgSQL_stmt_block": {
	        "lineno": 2,
	        "body": [
	          {
	            "PLpgSQL_stmt_perform": {
	              "lineno": 4,
	              "expr": {
	                "PLpgSQL_expr": {
	                  "query": "SELECT pg_advisory_lock(sender_id)",
	                  "parseMode": 0
	                }
	              }
	            }
	          },
	          {
				... similar to above
	          },
	          {
	            "PLpgSQL_stmt_execsql": {
	              "lineno": 8,
	              "sqlstmt": {
	                "PLpgSQL_expr": {
	                  "query": "UPDATE accounts \n    SET balance = balance - transfer_amount \n    WHERE account_id = sender_id",
	                  "parseMode": 0
	                }
	              }
	            }
	          },
	          {
				.... similar to above
	          },
	          {
	            ... similar to below
	          },
	          {
	            "PLpgSQL_stmt_perform": {
	              "lineno": 19,
	              "expr": {
	                "PLpgSQL_expr": {
	                  "query": "SELECT pg_advisory_unlock(receiver_id)",
	                  "parseMode": 0
	                }
	              }
	            }
	          },
	          {
	            "PLpgSQL_stmt_return": {}
	          }
	        ]
	      }
	    }
	  }
	}
*/
func TraversePlPgSQLActions(action interface{}, plPgSqlStatements *[]string) {
	actionMap, ok := action.(map[string]interface{})
	if !ok {
		//In case the value of a field is not a <key , val> but a list of <key, val> e.g. "body"
		lists, ok := action.([]interface{})
		if ok {
			for _, l := range lists {
				TraversePlPgSQLActions(l, plPgSqlStatements)
			}
		}
		return
	}

	for k, v := range actionMap {
		switch k {
		// base case of recursive calls to reach this PLPGSQL_EXPR field in json which will have "query" field with statement
		case PLPGSQL_EXPR: 
			expr, ok := v.(map[string]interface{})
			if ok {
				query, ok := expr[QUERY]
				if ok {
					q := formatExprQuery(query.(string)) // formating the query of parsed json if required

					*plPgSqlStatements = append(*plPgSqlStatements, q)
				}
			}
		default:
			TraversePlPgSQLActions(v, plPgSqlStatements)
		}
	}
}

// Function to format the PLPGSQL EXPR query from the json string
func formatExprQuery(q string) string {
	/*
	PLPGSQL line - 
	EXECUTE 'DROP TABLE IF EXISTS employees';

	json str - 
	 "PLpgSQL_expr": {
		"query": "'DROP TABLE IF EXISTS employees'",
	*/
	q = strings.Trim(q, "'") //to remove any extra '' around the statement
	q = strings.TrimSpace(q)
	if !strings.HasSuffix(q, ";") { // adding the ; to query in case not added already
		q += ";"
	}
	return q
}

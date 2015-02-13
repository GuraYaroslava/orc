package db

import (
    "database/sql"
    "fmt"
    _ "github.com/lib/pq"
    "github.com/orc/utils"
    "log"
    "strconv"
    "strings"
    "time"
)

var DB, _ = sql.Open(
    "postgres",
    "host=localhost"+
        " user="+user+
        " dbname="+dbname+
        " password="+password+
        " sslmode=disable")

func Exec(query string, params []interface{}) sql.Result {
    log.Println(query)
    stmt, err := DB.Prepare(query)
    utils.HandleErr("[queries.Exec] Prepare: ", err, nil)
    defer stmt.Close()
    result, err := stmt.Exec(params...)
    utils.HandleErr("[queries.Exec] Exec: ", err, nil)
    return result
}

func Query(query string, params []interface{}) []interface{} {
    log.Println(query)

    stmt, err := DB.Prepare(query)
    utils.HandleErr("[queries.Query] Prepare: ", err, nil)
    defer stmt.Close()
    rows, err := stmt.Query(params...)
    utils.HandleErr("[queries.Query] Query: ", err, nil)
    defer rows.Close()

    rowsInf := Exec(query, params)
    columns, _ := rows.Columns()
    size, err := rowsInf.RowsAffected()
    utils.HandleErr("[Entity.Select] RowsAffected: ", err, nil)

    return ConvertData(columns, size, rows)
}

func QueryRow(query string, params []interface{}) *sql.Row {
    log.Println(query)
    stmt, err := DB.Prepare(query)
    utils.HandleErr("[queries.QueryRow] Prepare: ", err, nil)
    defer stmt.Close()
    result := stmt.QueryRow(params...)
    utils.HandleErr("[queries.QueryRow] Query: ", err, nil)
    return result
}

func QueryCreateSecuence(tableName string) {
    Exec("CREATE SEQUENCE "+tableName+"_id_seq;", nil)
}

func QueryCreateTable(tableName string, fields []map[string]string) {
    QueryCreateSecuence(tableName)
    query := "CREATE TABLE IF NOT EXISTS %s ("
    for i := 0; i < len(fields); i++ {
        query += fields[i]["field"] + " "
        query += fields[i]["type"] + " "
        query += fields[i]["null"] + " "
        switch fields[i]["extra"] {
        case "PRIMARY":
            query += "PRIMARY KEY DEFAULT NEXTVAL('"
            query += tableName + "_id_seq'), "
            break
        case "REFERENCES":
            query += "REFERENCES " + fields[i]["refTable"] + "(" + fields[i]["refField"] + ") ON DELETE CASCADE, "
            break
        case "UNIQUE":
            query += "UNIQUE, "
            break
        default:
            query += ", "
        }
    }
    query = query[0 : len(query)-2]
    query += ");"
    Exec(fmt.Sprintf(query, tableName), nil)
}

func QuerySelect(tableName, where string, fields []string) string {
    query := "SELECT %s FROM %s"
    if where != "" {
        query += " WHERE %s;"
        return fmt.Sprintf(query, strings.Join(fields, ", "), tableName, where)
    } else {
        return fmt.Sprintf(query, strings.Join(fields, ", "), tableName)
    }
}

func QueryInsert(tableName string, fields []string, params []interface{}, extra string) *sql.Row {
    query := "INSERT INTO %s (%s) VALUES (%s) %s;"
    f := strings.Join(fields, ", ")
    p := strings.Join(MakeParams(len(fields)), ", ")
    return QueryRow(fmt.Sprintf(query, tableName, f, p, extra), params)
}

func QueryUpdate(tableName, where string, fields []string, params []interface{}) {
    query := "UPDATE %s SET %s WHERE %s;"
    p := strings.Join(MakePairs(fields), ", ")
    Exec(fmt.Sprintf(query, tableName, p, where), params)
}

func QueryDelete(tableName, fieldName string, valParams []interface{}) {
    query := "DELETE FROM %s WHERE %s IN (%s)"
    params := strings.Join(MakeParams(len(valParams)), ", ")
    Exec(fmt.Sprintf(query, tableName, fieldName, params), valParams)
}

func IsExists(tableName, fieldName string, value string) bool {
    var result string
    query := QuerySelect(tableName, fieldName+"=$1", []string{fieldName})
    row := QueryRow(query, []interface{}{value})
    err := row.Scan(&result)
    return err != sql.ErrNoRows
}

func IsExists_(tableName string, fields []string, params []interface{}) bool {
    query := "SELECT %s FROM %s WHERE %s;"
    f := strings.Join(fields, ", ")
    p := strings.Join(MakePairs(fields), " AND ")
    log.Println(fmt.Sprintf(query, f, tableName, p))
    var result string
    row := QueryRow(fmt.Sprintf(query, f, tableName, p), params)
    err := row.Scan(&result)
    return err != sql.ErrNoRows
}

func MakeParams(n int) []string {
    var result = make([]string, n)
    for i := 0; i < n; i++ {
        result[i] = "$" + strconv.Itoa(i+1)
    }
    return result
}

func MakePairs(fields []string) []string {
    var result = make([]string, len(fields))
    for i := 0; i < len(fields); i++ {
        result[i] = fields[i] + "=$" + strconv.Itoa(i+1)
    }
    return result
}

/**
 * condition: the AND condition and the OR condition
 * where: [fieldName1, paramVal1, fieldName2, paramVal2, ...]
 */
func Select(tableName string, where []string, condition string, fields []string) []interface{} {
    var key []string
    var val []interface{}
    var paramName = 1
    if len(where) != 0 {
        for i := 0; i < len(where)-1; i += 2 {
            key = append(key, where[i]+"=$"+strconv.Itoa(paramName))
            val = append(val, where[i+1])
            paramName++
        }
    }
    query := QuerySelect(tableName, strings.Join(key, " "+condition+" "), fields)
    return Query(query, val)
}

func ConvertData(columns []string, size int64, rows *sql.Rows) []interface{} {
    row := make([]interface{}, len(columns))
    values := make([]interface{}, len(columns))
    answer := make([]interface{}, size)

    for i, _ := range row {
        row[i] = &values[i]
    }

    j := 0
    for rows.Next() {
        rows.Scan(row...)
        record := make(map[string]interface{}, len(values))
        for i, col := range values {
            if col != nil {
                //fmt.Printf("\n%s: type= %s\n", columns[i], reflect.TypeOf(col))
                switch col.(type) {
                case bool:
                    record[columns[i]] = col.(bool)
                case int:
                    record[columns[i]] = col.(int)
                case int64:
                    record[columns[i]] = col.(int64)
                case float64:
                    record[columns[i]] = col.(float64)
                case string:
                    record[columns[i]] = col.(string)
                case []byte:
                    record[columns[i]] = string(col.([]byte))
                case []int8:
                    record[columns[i]] = col.([]string)
                case time.Time:
                    record[columns[i]] = col
                default:
                    utils.HandleErr("Entity.Select: Unexpected type.", nil, nil)
                }
            }
            answer[j] = record
        }
        j++
    }
    return answer
}

func InnerJoin(
    selectFields []string,

    fromTable string,
    fromTableRef string,
    fromField []string,

    joinTables []string,
    joinRef []string,
    joinField []string,

    where string) string {

    query := "SELECT "
    for i := 0; i < len(selectFields); i++ {
        query += selectFields[i] + ", "
    }
    query = query[0 : len(query)-2]
    query += " FROM " + fromTable + " " + fromTableRef
    for i := 0; i < len(joinTables); i++ {
        query += " INNER JOIN " + joinTables[i] + " " + joinRef[i]
        query += " ON " + joinRef[i] + "." + joinField[i] + " = " + fromTableRef + "." + fromField[i]
    }
    query += " " + where
    return query
}

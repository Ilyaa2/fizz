package translators

import (
	"context"
	"database/sql"
	"path"
	"strings"
	"time"

	"github.com/ydb-platform/ydb-go-sdk/v3"
	"github.com/ydb-platform/ydb-go-sdk/v3/table"

	"github.com/gobuffalo/fizz"
)

type ydbSchema struct {
	Schema
}

func (y *ydbSchema) Build() error {
	db, err := sql.Open("ydb", y.URL)
	if err != nil {
		return err
	}
	defer db.Close()

	nativeDriver, err := ydb.Unwrap(db)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer nativeDriver.Close(ctx)

	res, err := db.Query("SELECT Path FROM `.sys/partition_stats`;")
	if err != nil {
		return err
	}
	defer res.Close()

	for res.Next() {
		var fullPath string
		err = res.Scan(&fullPath)
		if err != nil {
			return err
		}
		tableName := strings.TrimPrefix(fullPath, nativeDriver.Name()+"/")
		//просматриваем только таблицы первого уровня, например /local/users, но в целом это отсеивание можно убрать
		if len(strings.Split(tableName, "/")) != 1 {
			continue
		}
		tbl := &fizz.Table{
			Columns: []fizz.Column{},
			Indexes: []fizz.Index{},
		}
		tbl.Name = tableName
		err = y.buildTableData(ctx, tbl, nativeDriver)
		if err != nil {
			return err
		}
	}

	return nil
}

func (y *ydbSchema) buildTableData(ctx context.Context, tbl *fizz.Table, nativeDriver *ydb.Driver) error {
	dbPath := path.Join(nativeDriver.Name(), tbl.Name)

	return nativeDriver.Table().Do(ctx,
		func(ctx context.Context, s table.Session) error {
			desc, err := s.DescribeTable(ctx, dbPath)
			if err != nil {
				return err
			}

			primaryKeys := desc.PrimaryKey
			for _, col := range desc.Columns {
				sanitizeType := func(input string) string {
					if strings.HasPrefix(input, "Optional<") && strings.HasSuffix(input, ">") {
						return input[len("Optional<") : len(input)-1]
					}
					return input
				}
				tbl.Columns = append(tbl.Columns, fizz.Column{
					Name:    col.Name,
					ColType: sanitizeType(col.Type.Yql()),
					Primary: isFieldPrimaryKey(primaryKeys, col.Name),
					Options: nil, //нет смысла
				})
			}

			for _, idx := range desc.Indexes {
				tbl.Indexes = append(tbl.Indexes, fizz.Index{
					Name:    idx.Name,
					Columns: idx.IndexColumns,
				})
			}

			y.schema[strings.TrimPrefix(dbPath, nativeDriver.Name()+"/")] = tbl
			return nil
		},
	)
}

func isFieldPrimaryKey(keys []string, field string) bool {
	for _, key := range keys {
		if field == key {
			return true
		}
	}
	return false
}

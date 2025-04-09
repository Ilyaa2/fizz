package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/gobuffalo/fizz"
	"github.com/ydb-platform/ydb-go-sdk/v3"
	"github.com/ydb-platform/ydb-go-sdk/v3/table"
	"log"
	"path"
	"strings"
	"time"
)

func main() {
	//db, err := sql.Open("ydb", "grpc://localhost:2136/local?query_mode=scripting") - работает
	//db, err := sql.Open("ydb", "grpc://localhost:2136/local?query_mode=query") - работает
	db, err := sql.Open("ydb", "grpc://localhost:2136/local?query_mode=query&go_query_bind=numeric,declare,table_path_prefix(/local)")
	if err != nil {
		fmt.Println(err)
	}
	//res, err := db.Exec("CREATE TABLE `schema_migration` (\n`version` Utf8 NOT NULL,\nPRIMARY KEY(`version`)\n);")
	res, err := db.Query("SELECT EXISTS (SELECT schema_migration.* FROM schema_migration AS schema_migration WHERE version = $1)", "21221232321")
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println(res)
	}
}

func mainр() {
	//db, err := sql.Open("ydb", "grpc://localhost:2135/local?go_query_bind=positional,declare")
	db, err := sql.Open("ydb", "grpc://localhost:2136/local")
	defer db.Close()
	if err != nil {
		fmt.Println(err)
	}

	nativeDriver, err := ydb.Unwrap(db)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	defer nativeDriver.Close(ctx)
	err = nativeDriver.Scheme().RemoveDirectory(context.Background(), "/local/pop_test")
	if err != nil {
		fmt.Println(err)
	}
	/*
		d, err := nativeDriver.Scheme().ListDirectory(context.Background(), "")
		if err != nil {
			fmt.Println(err)
		}

	*/
	/*
		err = nativeDriver.Scheme().MakeDirectory(context.Background(), "/local/pop_test2")
		if err != nil {
			fmt.Println(err)
		}

	*/
	r, err := nativeDriver.Query().Query(context.Background(), "select * from `pop_test/ydb_row_table`;")
	fmt.Println(r)
}

func maing() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	db, err := sql.Open("ydb", "grpc://localhost:2136/local")
	if err != nil {
		return err
	}
	defer db.Close()

	nativeDriver, err := ydb.Unwrap(db)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10000*time.Second)
	defer cancel()
	defer nativeDriver.Close(ctx)

	res, err := db.Query("SELECT OwnerId, Path FROM `.sys/partition_stats`;")
	if err != nil {
		return err
	}
	defer res.Close()

	dir, err := nativeDriver.Scheme().ListDirectory(ctx, "/local")
	if err != nil {
		return err
	}
	_ = dir
	endpoints, err := nativeDriver.Discovery().Discover(ctx)
	if err != nil {
		return err
	}
	whoami, err := nativeDriver.Discovery().WhoAmI(ctx)
	if err != nil {
		return err
	}
	_, _ = whoami, endpoints

	for res.Next() {
		var fullPath string
		err = res.Scan(&fullPath)
		if err != nil {
			return err
		}
		tableName := strings.TrimPrefix(fullPath, nativeDriver.Name()+"/")
		tbl := &fizz.Table{
			Columns: []fizz.Column{},
			Indexes: []fizz.Index{},
		}
		tbl.Name = tableName
		err = buildTableData(ctx, tbl, nativeDriver)
		if err != nil {
			return err
		}
	}

	//nativeDriver.Scheme().MakeDirectory()

	return nil
}

func buildTableData(ctx context.Context, tbl *fizz.Table, nativeDriver *ydb.Driver) error {
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

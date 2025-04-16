package translators

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"

	"github.com/ydb-platform/fizz"
)

var ErrUnimplemented = errors.New("this type of operation doesn't implemented in db as a sql query")

var RandInt = rand.Int
var TableInfo func(string) (*fizz.Table, error)

type Ydb struct {
	Schema SchemaQuery
}

func NewYdb(url string) *Ydb {
	schema := &ydbSchema{
		Schema{
			schema: map[string]*fizz.Table{},
			URL:    url,
		},
	}
	schema.Builder = schema
	y := &Ydb{Schema: schema}
	TableInfo = y.Schema.TableInfo
	return y
}

func (Ydb) Name() string {
	return "ydb"
}

func (y *Ydb) CreateTable(t fizz.Table) (string, error) {
	var sql []string
	var cols []string
	var s string
	for _, c := range t.Columns {
		if c.Primary {
			switch c.ColType {
			case "string":
				c.ColType = "Utf8"
			case "uuid":
				c.ColType = "Utf8"
			case "integer", "INT", "int":
				c.ColType = "Serial"
			case "bigint", "BIGINT":
				c.ColType = "BigSerial"
			default:
				return "", fmt.Errorf("can not use %s as a primary key", c.ColType)
			}
		}
		cols = append(cols, y.buildAddColumn(c, false))
		if c.Primary {
			cols = append(cols, fmt.Sprintf("PRIMARY KEY(`%s`)", c.Name))
		}
	}

	primaryKeys := t.PrimaryKeys()
	if len(primaryKeys) > 1 {
		pks := make([]string, len(primaryKeys))
		for i, pk := range primaryKeys {
			pks[i] = fmt.Sprintf("`%s`", pk)
		}
		cols = append(cols, fmt.Sprintf("PRIMARY KEY(%s)", strings.Join(pks, ", ")))
	}

	s = fmt.Sprintf("CREATE TABLE `%s` (\n%s\n);", t.Name, strings.Join(cols, ",\n"))
	sql = append(sql, s)

	for _, i := range t.Indexes {
		s, err := y.AddIndex(fizz.Table{
			Name:    t.Name,
			Indexes: []fizz.Index{i},
		})
		if err != nil {
			return "", err
		}
		sql = append(sql, s)
	}

	return strings.Join(sql, "\n"), nil
}

func (y *Ydb) DropTable(t fizz.Table) (string, error) {
	return fmt.Sprintf("DROP TABLE `%s`;", t.Name), nil
}

func (y *Ydb) RenameTable(t []fizz.Table) (string, error) {
	if len(t) != 2 {
		return "", fmt.Errorf("not enough or too much tables given")
	}
	return fmt.Sprintf("ALTER TABLE `%s` RENAME TO `%s`;", t[0].Name, t[1].Name), nil
}

func (y *Ydb) ChangeColumn(t fizz.Table) (string, error) {
	withIndices := true
	//в Ydb выдается ошибка при удалении колонки, если на ней были индексы.

	/*
		ALTER TABLE `users` ADD COLUMN `mycolumn3830129789844046230` String;
		UPDATE `users` SET `mycolumn383	0129789844046230` = `mycolumn`;

		-- эти операции нельзя выполнять в одной транзакции, так как во время исполнения второй команды первая команда еще не применилась
		-- так что их тоже похоже придется бить на разные части
		ALTER TABLE `users` DROP COLUMN `mycolumn`;
		ALTER TABLE `users` ADD COLUMN `mycolumn` String;
		--

		UPDATE `users` SET `mycolumn` = `mycolumn3830129789844046230`;
	*/
	if len(t.Columns) == 0 {
		return "", fmt.Errorf("not enough columns given")
	}
	actualizedTable, err := TableInfo(t.Name)
	if err != nil {
		withIndices = false
		log.Println("error from ydb Schema method TableInfo() while changing column: " + err.Error())
	}

	baseColumn := t.Columns[0]

	tmpColumn := fizz.Column{
		Name:    baseColumn.Name + strconv.Itoa(RandInt()),
		ColType: baseColumn.ColType, //тип выставляем такой же, как и у колонки, на которую меняем
		Primary: baseColumn.Primary,
		Options: baseColumn.Options,
	}

	//Schema нужна как раз для того, чтобы получить список индексов для колонки.
	var appropriateIndices []fizz.Index
	if withIndices {
		for _, idx := range actualizedTable.Indexes {
			if isColumnPresentInColumnsSet(idx.Columns, baseColumn.Name) {
				appropriateIndices = append(appropriateIndices, idx)
			}
		}
	}

	createTmpColumnSql := fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN %s;", t.Name, y.buildAddColumn(tmpColumn, true))
	copyToTmpColumnSql := fmt.Sprintf("UPDATE `%s` SET `%s` = `%s`;", t.Name, tmpColumn.Name, baseColumn.Name)

	var deleteIndicesInOldColumnSqls []string
	for _, idx := range appropriateIndices {
		deleteIndicesInOldColumnSqls = append(deleteIndicesInOldColumnSqls,
			fmt.Sprintf("ALTER TABLE `%s` DROP INDEX `%s`;", t.Name, idx.Name))
	}

	deleteOldColumnSql := fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN `%s`;", t.Name, baseColumn.Name)
	createNewColumnSql := fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN %s;", t.Name, y.buildAddColumn(baseColumn, true))

	var createIndicesInNewColumnSqls []string
	for _, idx := range appropriateIndices {
		createIndicesInNewColumnSqls = append(createIndicesInNewColumnSqls,
			fmt.Sprintf("ALTER TABLE `%s` ADD INDEX `%s` GLOBAL ON (%s);", t.Name, idx.Name, strings.Join(idx.Columns, ", ")))
	}

	copyToNewColumnSql := fmt.Sprintf("UPDATE `%s` SET `%s` = `%s`;", t.Name, baseColumn.Name, tmpColumn.Name)
	deleteTmpColumnSql := fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN `%s`;", t.Name, tmpColumn.Name)

	var finalSql []string
	//ddl
	finalSql = append(finalSql, createTmpColumnSql)
	//--

	//dml
	finalSql = append(finalSql, copyToTmpColumnSql)
	//--

	//ddl
	finalSql = append(finalSql, deleteIndicesInOldColumnSqls...)
	finalSql = append(finalSql, deleteOldColumnSql, createNewColumnSql)
	finalSql = append(finalSql, createIndicesInNewColumnSqls...)
	//--

	//dml
	finalSql = append(finalSql, copyToNewColumnSql)
	//--

	//dml
	finalSql = append(finalSql, deleteTmpColumnSql)
	return strings.Join(finalSql, "\n"), nil
}

func (y *Ydb) AddColumn(t fizz.Table) (string, error) {
	if len(t.Columns) == 0 {
		return "", fmt.Errorf("not enough columns given")
	}
	c := t.Columns[0]
	s := fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN %s;", t.Name, y.buildAddColumn(c, true))
	return s, nil
}

func (y *Ydb) DropColumn(t fizz.Table) (string, error) {
	if len(t.Columns) == 0 {
		return "", fmt.Errorf("not enough columns given")
	}
	return fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN `%s`;", t.Name, t.Columns[0].Name), nil
}

func (y *Ydb) RenameColumn(t fizz.Table) (string, error) {
	/*
		--меняем название first_name

		--ddl
		--добавили переименованную колонку
		ALTER TABLE `users` ADD COLUMN `copy_first_name` Utf8;

		--добавили на нее индексы
		ALTER TABLE `users` ADD INDEX `users_name_idx` GLOBAL ON (`first_name`);
		ALTER TABLE `users` ADD INDEX `users_first_last_name_index` GLOBAL ON (`first_name`, `last_name`);


		--dml для сохранения данных
		UPDATE `users` SET `copy_first_name` = `first_name`;

		--ddl
		ALTER TABLE `users` DROP INDEX `users_name_idx`;
		ALTER TABLE `users` DROP INDEX `users_first_last_name_index`;

		ALTER TABLE `users` DROP COLUMN `first_name`;
	*/

	//в Ydb выдается ошибка при удалении колонки, если на ней были индексы.
	if len(t.Columns) == 0 {
		return "", fmt.Errorf("not enough columns given")
	}
	actualizedTable, err := TableInfo(t.Name)
	if err != nil {
		return "", err
	}
	actualizedColumn, ok := FindColumnInfo(actualizedTable, t.Columns[0].Name)
	if !ok {
		return "", fmt.Errorf("couldn't find the type of the column: %s in renaming column method", t.Columns[0].Name)
	}

	baseColumn := t.Columns[0]
	newFizzColumn := t.Columns[1]

	newColumn := fizz.Column{
		Name:    newFizzColumn.Name, //меняем только имя
		ColType: actualizedColumn.ColType,
		Primary: baseColumn.Primary,
		Options: baseColumn.Options,
	}

	createNewColumnSql := fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN %s;", t.Name, y.buildAddColumn(newColumn, true))

	//Schema нужна как раз для того, чтобы получить список индексов для колонки.
	var appropriateIndices []fizz.Index
	for _, idx := range actualizedTable.Indexes {
		if isColumnPresentInColumnsSet(idx.Columns, baseColumn.Name) {
			appropriateIndices = append(appropriateIndices, idx)
		}
	}

	var createIndicesInNewColumnSqls []string
	for _, idx := range appropriateIndices {
		createIndicesInNewColumnSqls = append(createIndicesInNewColumnSqls,
			fmt.Sprintf("ALTER TABLE `%s` ADD INDEX `%s` GLOBAL ON (%s);", t.Name, idx.Name,
				strings.Join(updateColumnInColumnList(idx.Columns, baseColumn.Name, newColumn.Name), ", ")))
	}
	//ddl
	copyToNewColumnSql := fmt.Sprintf("UPDATE `%s` SET `%s` = `%s`;", t.Name, newColumn.Name, baseColumn.Name)

	var deleteIndicesInOldColumnSqls []string
	for _, idx := range appropriateIndices {
		deleteIndicesInOldColumnSqls = append(deleteIndicesInOldColumnSqls,
			fmt.Sprintf("ALTER TABLE `%s` DROP INDEX `%s`;", t.Name, idx.Name))
	}

	deleteOldColumnSql := fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN `%s`;", t.Name, baseColumn.Name)

	var finalSql []string
	//ddl
	finalSql = append(finalSql, createNewColumnSql)
	finalSql = append(finalSql, deleteIndicesInOldColumnSqls...)
	finalSql = append(finalSql, createIndicesInNewColumnSqls...)
	//--

	//dml
	finalSql = append(finalSql, copyToNewColumnSql)
	//--

	//ddl
	finalSql = append(finalSql, deleteOldColumnSql)
	//--
	return strings.Join(finalSql, "\n"), nil
}

func (y *Ydb) AddIndex(t fizz.Table) (string, error) {
	if len(t.Indexes) == 0 {
		return "", fmt.Errorf("not enough indexes given")
	}
	i := t.Indexes[0]
	var cols []string
	for _, c := range i.Columns {
		cols = append(cols, fmt.Sprintf("`%s`", c))
	}

	s := fmt.Sprintf("ALTER TABLE `%s` ADD INDEX `%s` GLOBAL ON (%s);", t.Name, i.Name, strings.Join(cols, ", "))
	if i.Unique {
		s = strings.Replace(s, "GLOBAL", "GLOBAL UNIQUE", 1)
	}
	return s, nil
}

func (y *Ydb) DropIndex(t fizz.Table) (string, error) {
	if len(t.Indexes) == 0 {
		return "", fmt.Errorf("not enough indexes given")
	}
	return fmt.Sprintf("ALTER TABLE `%s` DROP INDEX `%s`;", t.Name, t.Indexes[0].Name), nil
}

func (y *Ydb) RenameIndex(t fizz.Table) (string, error) {
	if len(t.Indexes) < 2 {
		return "", fmt.Errorf("not enough indexes given")
	}
	return fmt.Sprintf("ALTER TABLE `%s` RENAME INDEX `%s` TO `%s`;", t.Name, t.Indexes[0].Name, t.Indexes[1].Name), nil
}

func (y *Ydb) AddForeignKey(_ fizz.Table) (string, error) {
	return "", ErrUnimplemented
}

func (y *Ydb) DropForeignKey(_ fizz.Table) (string, error) {
	return "", ErrUnimplemented
}

func (y *Ydb) buildAddColumn(c fizz.Column, isAddColumn bool) string {
	s := fmt.Sprintf("`%s` %s", c.Name, y.colType(c))
	if !isAddColumn && (c.Options["null"] == nil || c.Primary) {
		s = fmt.Sprintf("%s NOT NULL", s)
	}

	if !isAddColumn && c.Options["default"] != nil {
		_, ok := c.Options["default"].(string)
		if ok {
			if _, err := strconv.ParseFloat(c.Options["default"].(string), 64); err != nil {
				return fmt.Sprintf("%s DEFAULT '%v'", s, c.Options["default"])
			}
		}
		s = fmt.Sprintf("%s DEFAULT %v", s, c.Options["default"])
	}

	return s
}

func (y *Ydb) colType(c fizz.Column) string {
	switch c.ColType {
	case "Uuid", "uuid", "string", "text":
		return "Utf8"
	case "bool", "boolean":
		return "Bool"
	case "time", "timestamp":
		return "Timestamp"
	case "datetime":
		return "Datetime"
	case "blob", "[]byte":
		return "String"
	case "float":
		return "Float"
	case "double", "numeric":
		return "Double"
	case "decimal":
		if c.Options["precision"] != nil && c.Options["scale"] != nil {
			return fmt.Sprintf("Decimal(%d,%d)", c.Options["precision"], c.Options["scale"])
		}
		return "Double"
	case "int", "integer":
		return "Int64"
	case "json":
		return "Json"
	default:
		return c.ColType
	}
}

func updateColumnInColumnList(columns []string, skipColumn string, fillColumn string) []string {
	var colList []string
	for _, col := range columns {
		if col == skipColumn {
			colList = append(colList, fillColumn)
		} else {
			colList = append(colList, col)
		}
	}
	return colList
}

func isColumnPresentInColumnsSet(columns []string, columnName string) bool {
	for _, col := range columns {
		if col == columnName {
			return true
		}
	}
	return false
}

package translators

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gobuffalo/fizz"
)

var ErrUnimplemented = errors.New("this type of operation doesn't implemented in db as a sql query")

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
	return &Ydb{Schema: schema}
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
				c.ColType = "Uuid"
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

func (y *Ydb) ChangeColumn(_ fizz.Table) (string, error) {
	return "", ErrUnimplemented
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

func (y *Ydb) RenameColumn(_ fizz.Table) (string, error) {
	return "", ErrUnimplemented
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
			s = fmt.Sprintf("%s DEFAULT '%v'", s, c.Options["default"])
		} else {
			s = fmt.Sprintf("%s DEFAULT %v", s, c.Options["default"])
		}
	}

	return s
}

func (y *Ydb) colType(c fizz.Column) string {
	switch c.ColType {
	case "string", "text":
		return "Utf8"
	case "uuid":
		return "Uuid"
	case "bool":
		return "Bool"
	case "time", "timestamp":
		return "Timestamp"
	case "datetime":
		return "Datetime"
	case "blob", "[]byte":
		return "String"
	case "float":
		return "Float"
	case "double":
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

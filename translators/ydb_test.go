package translators_test

import (
	"fmt"
	"github.com/gobuffalo/fizz"

	"github.com/gobuffalo/fizz/translators"
)

var _ fizz.Translator = (*translators.Ydb)(nil)
var ydbt = translators.NewYdb("")

func init() {
	u := "%s://%s:%s/%s"
	//grpc://localhost:2136/local
	u = fmt.Sprintf(u, getEnv("YDB_PROTOCOL", "grpc"), getEnv("YDB_HOST", "localhost"),
		getEnv("MYSQL_PORT", "2136"), getEnv("YDB_PATH", "local"))
	ydbt = translators.NewYdb(u)
}

func (y *YdbSuite) Test_YDB_CreateTable() {
	r := y.Require()
	/*
		CREATE TABLE `users` (
		`id` Serial NOT NULL,
		PRIMARY KEY(`id`),
		`first_name` Utf8 NOT NULL,
		`last_name` Utf8 NOT NULL,
		`email` Utf8 NOT NULL DEFAULT 'some@exam.com',
		`permissions` Json,
		`age` Int64 DEFAULT 40,
		`raw` String NOT NULL,
		`company_id` Uuid NOT NULL,
		`created_at` Timestamp NOT NULL,
		`updated_at` Timestamp NOT NULL
		);
	*/
	ddl := "CREATE TABLE `users` (" +
		"\n`id` Serial NOT NULL," +
		"\nPRIMARY KEY(`id`)," +
		"\n`first_name` Utf8 NOT NULL," +
		"\n`last_name` Utf8 NOT NULL," +
		"\n`email` Utf8 NOT NULL DEFAULT 'some@exam.com'," +
		"\n`permissions` Json," +
		"\n`age` Int64 DEFAULT 40," +
		"\n`raw` String NOT NULL," +
		"\n`company_id` Uuid NOT NULL," +
		"\n`created_at` Timestamp NOT NULL," +
		"\n`updated_at` Timestamp NOT NULL\n);"

	res, _ := fizz.AString(`
	create_table("users") {
		t.Column("id", "integer", {"primary": true})
		t.Column("first_name", "string", {})
		t.Column("last_name", "string", {})
		t.Column("email", "string", {"size":20, "default": "some@exam.com"})
		t.Column("permissions", "json", {"null": true})
		t.Column("age", "integer", {"null": true, "default": 40})
		t.Column("raw", "blob", {})
		t.Column("company_id", "uuid", {"default_raw": "uuid_generate_v1()"})
	}
	`, ydbt)
	r.Equal(ddl, res)
}

func (y *YdbSuite) Test_YDB_CreateTable_UUID() {
	r := y.Require()
	/*
		CREATE TABLE `users` (
		`first_name` Utf8 NOT NULL,
		`last_name` Utf8 NOT NULL,
		`email` Utf8 NOT NULL,
		`permissions` Json,
		`age` Int64 DEFAULT 40,
		`integer` Int64 NOT NULL,
		`float` Float NOT NULL,
		`bytes` String NOT NULL,
		`jason` Json NOT NULL,
		`mydecimal` Double NOT NULL,
		`mydecimal2` Decimal(22,9) NOT NULL,
		`uuid` Uuid NOT NULL,
		PRIMARY KEY(`uuid`),
		`created_at` Timestamp NOT NULL,
		`updated_at` Timestamp NOT NULL
		);
	*/
	ddl := "CREATE TABLE `users` (" +
		"\n`first_name` Utf8 NOT NULL," +
		"\n`last_name` Utf8 NOT NULL," +
		"\n`email` Utf8 NOT NULL," +
		"\n`permissions` Json," +
		"\n`age` Int64 DEFAULT 40," +
		"\n`integer` Int64 NOT NULL," +
		"\n`float` Float NOT NULL," +
		"\n`bytes` String NOT NULL," +
		"\n`jason` Json NOT NULL," +
		"\n`mydecimal` Double NOT NULL," +
		"\n`mydecimal2` Decimal(22,9) NOT NULL," +
		"\n`uuid` Uuid NOT NULL," +
		"\nPRIMARY KEY(`uuid`)," +
		"\n`created_at` Timestamp NOT NULL," +
		"\n`updated_at` Timestamp NOT NULL\n);"

	res, _ := fizz.AString(`
	create_table("users") {
		t.Column("first_name", "string", {})
		t.Column("last_name", "string", {})
		t.Column("email", "string", {"size":20})
		t.Column("permissions", "json", {"null": true})
		t.Column("age", "integer", {"null": true, "default": 40})
		t.Column("integer", "integer", {})
		t.Column("float", "float", {})
		t.Column("bytes", "[]byte", {})
		t.Column("jason", "json", {})
		t.Column("mydecimal", "decimal", {})
		t.Column("mydecimal2", "decimal", {"precision": 22, "scale": 9})
		t.Column("uuid", "uuid", {"primary": true})
	}
	`, ydbt)
	r.Equal(ddl, res)
}

func (y *YdbSuite) Test_Ydb_Disable_TimeStamps() {
	r := y.Require()
	/*
		CREATE TABLE `users` (
		`id` Serial NOT NULL,
		PRIMARY KEY(`id`)
		);
	*/
	ddl := "CREATE TABLE `users` (" +
		"\n`id` Serial NOT NULL," +
		"\nPRIMARY KEY(`id`)\n);"

	res, _ := fizz.AString(`
	create_table("users") {
		t.Column("id", "int", {"primary": true, "default_raw": "uuid_generate_v4()"})
		t.DisableTimestamps()
	}
	`, ydbt)
	r.Equal(ddl, res)
}

func (y *YdbSuite) Test_YDB_CreateTable_Cant_Set_PK_To_Nullable() {
	r := y.Require()
	/*
		CREATE TABLE `users` (
		`id` Serial NOT NULL,
		PRIMARY KEY(`id`)
		);
	*/
	ddl := "CREATE TABLE `users` (" +
		"\n`id` Serial NOT NULL," +
		"\nPRIMARY KEY(`id`)\n);"

	res, _ := fizz.AString(`
	create_table("users") {
		t.Column("id", "int", {"primary": true, "null": true})
		t.DisableTimestamps()
	}
	`, ydbt)
	r.Equal(ddl, res)
}

func (y *YdbSuite) Test_YDB_CreateTables_WithEmptyForeignKey() {
	r := y.Require()
	/*
		CREATE TABLE `users` (
		`id` Serial NOT NULL,
		PRIMARY KEY(`id`),
		`email` Utf8 NOT NULL,
		`created_at` Timestamp NOT NULL,
		`updated_at` Timestamp NOT NULL
		);
		CREATE TABLE `profiles` (
		`id` Serial NOT NULL,
		PRIMARY KEY(`id`),
		`user_id` INT NOT NULL,
		`first_name` Utf8 NOT NULL,
		`last_name` Utf8 NOT NULL,
		`created_at` Timestamp NOT NULL,
		`updated_at` Timestamp NOT NULL
		);
	*/
	ddl := "CREATE TABLE `users` (" +
		"\n`id` Serial NOT NULL," +
		"\nPRIMARY KEY(`id`)," +
		"\n`email` Utf8 NOT NULL," +
		"\n`created_at` Timestamp NOT NULL," +
		"\n`updated_at` Timestamp NOT NULL\n);" +
		"\n" +
		"CREATE TABLE `profiles` (" +
		"\n`id` Serial NOT NULL," +
		"\nPRIMARY KEY(`id`)," +
		"\n`user_id` INT NOT NULL," +
		"\n`first_name` Utf8 NOT NULL," +
		"\n`last_name` Utf8 NOT NULL," +
		"\n`created_at` Timestamp NOT NULL," +
		"\n`updated_at` Timestamp NOT NULL\n);"

	res, err := fizz.AString(`
	create_table("users") {
		t.Column("id", "INT", {"primary": true})
		t.Column("email", "string", {"size":20})
	}
	create_table("profiles") {
		t.Column("id", "INT", {"primary": true})
		t.Column("user_id", "INT", {})
		t.Column("first_name", "string", {})
		t.Column("last_name", "string", {})
		t.ForeignKey("user_id", {"users": ["id"]}, {})
	}
	`, ydbt)
	r.NoError(err)
	r.Equal(ddl, res)
}

func (y *YdbSuite) Test_YDB_CreateTables_WithCompositePrimaryKey() {
	r := y.Require()
	/*
		CREATE TABLE `user_profiles` (
		`user_id` INT NOT NULL,
		`profile_id` INT NOT NULL,
		`created_at` Timestamp NOT NULL,
		`updated_at` Timestamp NOT NULL,
		PRIMARY KEY(`user_id`, `profile_id`)
		);
	*/
	ddl := "CREATE TABLE `user_profiles` (" +
		"\n`user_id` INT NOT NULL," +
		"\n`profile_id` INT NOT NULL," +
		"\n`created_at` Timestamp NOT NULL," +
		"\n`updated_at` Timestamp NOT NULL," +
		"\nPRIMARY KEY(`user_id`, `profile_id`)\n);"

	res, _ := fizz.AString(`
	create_table("user_profiles") {
		t.Column("user_id", "INT")
		t.Column("profile_id", "INT")
		t.PrimaryKey("user_id", "profile_id")
	}
	`, ydbt)
	r.Equal(ddl, res)
}

func (y *YdbSuite) Test_YDB_DropTable() {
	r := y.Require()

	ddl := "DROP TABLE `users`;"

	res, _ := fizz.AString(`drop_table("users")`, ydbt)
	r.Equal(ddl, res)
}

func (y *YdbSuite) Test_YDB_RenameTable() {
	r := y.Require()

	ddl := "ALTER TABLE `users` RENAME TO `people`;"

	res, _ := fizz.AString(`rename_table("users", "people")`, ydbt)
	r.Equal(ddl, res)
}

func (y *YdbSuite) Test_YDB_RenameTable_NotEnoughValues() {
	r := y.Require()

	_, err := ydbt.RenameTable([]fizz.Table{})
	r.Error(err)
}

func (y *YdbSuite) Test_YDB_ChangeColumn() {
	r := y.Require()
	translators.RandInt = func() int {
		return 1
	}
	translators.TableInfo = func(s string) (*fizz.Table, error) {
		return &fizz.Table{
			Indexes: []fizz.Index{},
		}, nil
	}
	ddl := "ALTER TABLE `users` ADD COLUMN `mycolumn1` String;\n" +
		"UPDATE `users` SET `mycolumn1` = `mycolumn`;\n" +
		"ALTER TABLE `users` DROP COLUMN `mycolumn`;\n" +
		"ALTER TABLE `users` ADD COLUMN `mycolumn` String;\n" +
		"UPDATE `users` SET `mycolumn` = `mycolumn1`;\n" +
		"ALTER TABLE `users` DROP COLUMN `mycolumn1`;"
	res, err := fizz.AString(`change_column("users", "mycolumn", "[]byte", {"default": "foo", "size": 50})`, ydbt)
	r.NoError(err)
	r.Equal(ddl, res)
}

func (y *YdbSuite) Test_YDB_AddColumn_WithNoDefault() {
	r := y.Require()
	ddl := "ALTER TABLE `mytable` ADD COLUMN `mycolumn` Utf8;"

	res, _ := fizz.AString(`add_column("mytable", "mycolumn", "string", {})`, ydbt)

	r.Equal(ddl, res)
}

func (y *YdbSuite) Test_YDB_DropColumn() {
	r := y.Require()
	ddl := "ALTER TABLE `table_name` DROP COLUMN `column_name`;"

	res, _ := fizz.AString(`drop_column("table_name", "column_name")`, ydbt)

	r.Equal(ddl, res)
}

func (y *YdbSuite) Test_YDB_RenameColumn() {
	r := y.Require()
	translators.TableInfo = func(s string) (*fizz.Table, error) {
		return &fizz.Table{
			Name: "users",
			Columns: []fizz.Column{
				{
					Name:    "mycolumn",
					ColType: "String",
				},
			},
		}, nil
	}

	ddl := "ALTER TABLE `users` ADD COLUMN `new_column` String;\n" +
		"UPDATE `users` SET `new_column` = `mycolumn`;\n" +
		"ALTER TABLE `users` DROP COLUMN `mycolumn`;"
	res, err := fizz.AString(`rename_column("users", "mycolumn", "new_column")`, ydbt)
	r.NoError(err)
	r.Equal(ddl, res)
}

func (y *YdbSuite) Test_YDB_AddIndex() {
	r := y.Require()
	ddl := "ALTER TABLE `table_name` ADD INDEX `table_name_column_name_idx` GLOBAL ON (`column_name`);"

	res, _ := fizz.AString(`add_index("table_name", "column_name", {})`, ydbt)
	r.Equal(ddl, res)
}

func (y *YdbSuite) Test_YDB_AddIndex_Unique() {
	r := y.Require()
	ddl := "ALTER TABLE `table_name` ADD INDEX `table_name_column_name_idx` GLOBAL UNIQUE ON (`column_name`);"

	res, _ := fizz.AString(`add_index("table_name", "column_name", {"unique": true})`, ydbt)
	r.Equal(ddl, res)
}

func (y *YdbSuite) Test_YDB_AddIndex_MultiColumn() {
	r := y.Require()
	ddl := "ALTER TABLE `table_name` ADD INDEX `table_name_col1_col2_col3_idx` GLOBAL ON (`col1`, `col2`, `col3`);"

	res, _ := fizz.AString(`add_index("table_name", ["col1", "col2", "col3"], {})`, ydbt)
	r.Equal(ddl, res)
}

func (y *YdbSuite) Test_YDB_AddIndex_CustomName() {
	r := y.Require()
	ddl := "ALTER TABLE `table_name` ADD INDEX `custom_name` GLOBAL ON (`column_name`);"

	res, _ := fizz.AString(`add_index("table_name", "column_name", {"name": "custom_name"})`, ydbt)
	r.Equal(ddl, res)
}

func (y *YdbSuite) Test_YDB_DropIndex() {
	r := y.Require()
	ddl := "ALTER TABLE `users` DROP INDEX `my_idx`;"

	res, _ := fizz.AString(`drop_index("users", "my_idx")`, ydbt)
	r.Equal(ddl, res)
}

func (y *YdbSuite) Test_YDB_RenameIndex() {
	r := y.Require()

	ddl := "ALTER TABLE `table` RENAME INDEX `old_ix` TO `new_ix`;"

	res, _ := fizz.AString(`rename_index("table", "old_ix", "new_ix")`, ydbt)
	r.Equal(ddl, res)
}

func (y *YdbSuite) Test_YDB_AddForeignKey_Unimplemented() {
	r := y.Require()

	res, err := fizz.AString(`add_foreign_key("profiles", "user_id", {"users": ["id"]}, {})`, ydbt)
	r.ErrorIs(err, translators.ErrUnimplemented)
	r.Empty(res)
}

func (y *YdbSuite) Test_YDB_DropForeignKey_Unimplemented() {
	r := y.Require()

	res, err := fizz.AString(`drop_foreign_key("profiles", "profiles_users_id_fk", {})`, ydbt)
	r.ErrorIs(err, translators.ErrUnimplemented)
	r.Empty(res)
}

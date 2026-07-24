CREATE TABLE IF NOT EXISTS {{.TableName}} (
    id {{.SQLPrimaryKey}},
{{- range .Fields}}
    {{.ColumnName}} {{.SQLType}}{{if .Required}} NOT NULL{{end}},
{{- end}}
    created_at {{.SQLTimestamp}} NOT NULL,
    updated_at {{.SQLTimestamp}} NOT NULL
);
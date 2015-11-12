##

A simple Go program to create data dictionary's (field aggregation analysis) for table columns in a database.


### Installation

`
GOOS=linux GOARCH=386 CGO_ENABLED=0 go build -o bin/ddg.linux main.go
`

### Usage
`
ddg <config.json> <outfile.xlsx>
`

### Configuration

```javascript
{
  // Database information
  "db" : {
    "host" : "localhost",
    "port" : 5432,
    "user" : "postgres",
    "password" : "testpass",
    "database" : "postgres"
  },
  // Breakouts on a column
  "groupings" : [
    {
      "column" : "estimated_age",
      "as"     : "Estimated Age",
      "table"  : "us_consumer.consumers",
      "sheet"  : "Est_Age", // Specify the sheet to output the breakout
      "group"  : true // Flag should be true, for a breakout
    },
    // Single Count with where statement
    {
      "column" : "purchase_date",
      "as"     : "Has Puchase Date",
      "table"  : "us_consumer.consumers",
      "sheet"  : "Mortage",
      "condition" : "purchase_date IS NOT NULL"
    }
  ]
}
```

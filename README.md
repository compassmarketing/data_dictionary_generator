##

A simple Go program to create data dictionary's (field aggregation analysis) for table columns in a database.


### Installation

`
GOOS=linux GOARCH=386 CGO_ENABLED=0 go build -o bin/ddg.linux main.go
`

### Usage
`
ddg -c <config.json> [-l layoutFile] [-s test:password] [-r release] <outfile.xlsx>
-c - Configuration file
-l - Layout excel file to attach to email
-s - Email credentials for Gmail server
-r - Release name
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
      "sheet"  : "Homeowner",
      "statement" : "SELECT home_owner AS Home_Owner, count(*) as Count FROM us_consumer.consumers  GROUP BY home_owner  ORDER BY home_owner ASC ",
      "group"  : true,
      "mappings" : [
        "1=Yes",
        "0=No"
      ]
    }
    // Single Count with where statement
    {
      "statement": "SELECT count(*) as Total_Number_of_records_with_Phones FROM us_consumer.consumers c INNER JOIN us_consumer.emails e ON c.id = e.consumer_id WHERE phone IS NOT NULL",
      "sheet"  : "Other_Counts"
    },
    // Format Cells
    {
      "statement": "SELECT count(*) as Total FROM us_consumer.consumers c INNER JOIN us_consumer.emails e ON c.id = e.consumer_id WHERE phone IS NOT NULL",
      "sheet"  : "Other_Counts",
      "format" : {
        "Total" : "code"
      }
    },
  ]
}
```

package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/tealeg/xlsx"
	"io/ioutil"
	"log"
	"os"
)

//Columns
type grouping struct {
	Table     string `json:"table"`
	Column    string `json:"column"`
	As        string `json:"as"`
	Sheet     string `json:"sheet"`
	Condition string `json:"condition"`
	Group     bool   `json:"group"`
	Join      struct {
		Table string `json:"table"`
		On    string `json:"on"`
	} `json:"join"`
	Mappings []string `json:"mappings"`
}

// Options
type options struct {
	DB struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Database string `json:"database"`
		User     string `json:"user"`
		Password string `json:"password"`
	} `json:"db"`
	Groupings []grouping `json:"groupings"`
}

type breakout struct {
	name  []byte
	count int
}

var dd *xlsx.File

func empty(str string) bool {
	return len(str) == 0
}

// Make sure all required options are passed
func check(opts options) error {

	if empty(opts.DB.Host) {
		return errors.New("No postgres host found in options.")
	}

	if opts.DB.Port == 0 {
		return errors.New("No postgres dataportbase found in options.")
	}

	if empty(opts.DB.User) {
		return errors.New("No postgres user found in options.")
	}

	if empty(opts.DB.Database) {
		return errors.New("No postgres database found in options.")
	}

	if empty(opts.DB.Password) {
		return errors.New("No postgres password found in options.")
	}

	return nil
}

func getActiveSheet(sheet string) (*xlsx.Sheet, error) {
	var activeSheet *xlsx.Sheet
	var err error

	for _, sh := range dd.Sheets {
		if sh.Name == sheet {
			activeSheet = sh
		}
	}

	if activeSheet == nil {
		activeSheet, err = dd.AddSheet(sheet)
		if err != nil {
			return nil, err
		}
	}

	return activeSheet, nil
}

func writeRowToSheet(breakOt breakout, sheet string) error {

	activeSheet, err := getActiveSheet(sheet)

	if err != nil {
		return err
	}

	var cell *xlsx.Cell

	row := activeSheet.AddRow()
	cell = row.AddCell()
	if len(breakOt.name) > 0 {
		cell.Value = string(breakOt.name)
	} else {
		cell.Value = "Unknown"
	}
	cell = row.AddCell()
	cell.SetFloatWithFormat(float64(breakOt.count), "#,##0")

	return nil
}

func writeHeaderRowToSheet(name string, sheet string) error {

	activeSheet, err := getActiveSheet(sheet)

	if err != nil {
		return err
	}

	headerFont := xlsx.NewFont(12, "Verdana")
	headerFont.Bold = true
	headerFont.Underline = true
	headerStyle := xlsx.NewStyle()
	headerStyle.Font = *headerFont

	var cell *xlsx.Cell

	row := activeSheet.AddRow()
	cell = row.AddCell()
	cell.SetStyle(headerStyle)
	cell.Value = name
	activeSheet.AddRow()
	return nil
}

func writeFooterRowToSheet(total int64, sheet string) error {
	activeSheet, err := getActiveSheet(sheet)

	if err != nil {
		return err
	}

	footerFont := xlsx.NewFont(12, "Verdana")
	footerFont.Bold = true
	footerStyle := xlsx.NewStyle()
	footerStyle.Font = *footerFont

	var cell *xlsx.Cell
	row := activeSheet.AddRow()
	cell = row.AddCell()
	cell.SetStyle(footerStyle)
	cell.Value = "Total"
	cell = row.AddCell()
	cell.SetStyle(footerStyle)
	cell.SetFloatWithFormat(float64(total), "#,##0")

	activeSheet.AddRow()
	activeSheet.AddRow()

	return nil

}

func writeMappingsToSheet(mappings []string, sheet string) error {
	activeSheet, err := getActiveSheet(sheet)

	if err != nil {
		return err
	}

	for _, mapping := range mappings {
		row := activeSheet.AddRow()
		cell := row.AddCell()
		cell.Value = mapping
	}

	activeSheet.AddRow()
	activeSheet.AddRow()

	return nil
}

func main() {

	if len(os.Args) < 2 {
		log.Fatalln("Usage: ddg <config.json> <dd_name.xlsx>")
	}

	//Load Input File
	file, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		log.Fatalln(err.Error())
	}

	// Load options
	var opts options
	if err := json.Unmarshal(file, &opts); err != nil {
		log.Fatalln(err.Error())
	}

	// Check options for required
	if err := check(opts); err != nil {
		log.Fatalln(err.Error())
	}

	// Create File
	dd = xlsx.NewFile()

	// Open DB connection
	db, err := sql.Open("postgres", fmt.Sprintf("host=%s port=%d dbname=%s user=%s sslmode=disable password=%s", opts.DB.Host, opts.DB.Port, opts.DB.Database, opts.DB.User, opts.DB.Password))
	if err != nil {
		log.Fatalln(err.Error())
	}

	for _, group := range opts.Groupings {

		fmt.Printf("Getting breakout for: %s...", group.As)

		//Generate staement for breakout
		var statement bytes.Buffer
		if group.Group {
			statement.WriteString(fmt.Sprintf(` SELECT %s AS "%s", count(*) `, group.Column, group.As))
		} else {
			statement.WriteString(fmt.Sprintf(` SELECT count(*) `))
		}

		statement.WriteString(fmt.Sprintf(` FROM %s `, group.Table))

		if !empty(group.Join.Table) && !empty(group.Join.On) {
			statement.WriteString(fmt.Sprintf(` INNER JOIN %s ON %s `, group.Join.Table, group.Join.On))
		}

		if !empty(group.Condition) {
			statement.WriteString(fmt.Sprintf(` WHERE %s `, group.Condition))
		}

		if group.Group {
			statement.WriteString(fmt.Sprintf(` GROUP BY %s `, group.Column))
			statement.WriteString(fmt.Sprintf(` ORDER BY %s ASC `, group.Column))
		}

		rows, err := db.Query(string(statement.Bytes()))
		if err != nil {
			log.Fatalln(err.Error())
		}

		var total int64

		// Write Header
		if group.Group {
			err = writeHeaderRowToSheet(group.As, group.Sheet)
			if err != nil {
				log.Fatalln(err.Error())
			}
		}

		defer rows.Close()

		for rows.Next() {

			var breakO breakout
			if group.Group {
				if err := rows.Scan(&breakO.name, &breakO.count); err != nil {
					fmt.Println(err)
				}
			} else {
				breakO.name = bytes.NewBufferString(group.As).Bytes()
				if err := rows.Scan(&breakO.count); err != nil {
					fmt.Println(err)
				}
			}

			err = writeRowToSheet(breakO, group.Sheet)
			if err != nil {
				log.Fatalln(err.Error())
			}

			total += int64(breakO.count)

		}
		if err := rows.Err(); err != nil {
			log.Fatalln(err.Error())
		}

		if group.Group {
			writeFooterRowToSheet(total, group.Sheet)
		}

		if len(group.Mappings) > 0 {
			writeMappingsToSheet(group.Mappings, group.Sheet)
		}

		fmt.Println("done")

	}

	// Format sheets to show complete values
	for _, sheet := range dd.Sheets {
		sheet.SetColWidth(0, 1, 30)
	}

	// Export excel file
	err = dd.Save(os.Args[2])
	if err != nil {
		log.Fatalln(err.Error())
	}
}

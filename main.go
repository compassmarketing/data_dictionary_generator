package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/jordan-wright/email"
	_ "github.com/lib/pq"
	"github.com/tealeg/xlsx"
	"io/ioutil"
	"log"
	"net/smtp"
	"strconv"
	"strings"
)

//Columns
type grouping struct {
	Statement string            `json:"statement"`
	Sheet     string            `json:"sheet"`
	Group     bool              `json:"group"`
	Mappings  []string          `json:"mappings"`
	Format    map[string]string `json:"format"`
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

func writeRowToSheet(data [][]byte, sheet string, format map[string]string) ([]int, error) {

	activeSheet, err := getActiveSheet(sheet)

	if err != nil {
		return nil, err
	}

	var cell *xlsx.Cell
	counts := make([]int, len(data))

	row := activeSheet.AddRow()
	for idx, bytes := range data {
		var value string

		if bytes != nil {
			value = string(bytes)
		}

		cell = row.AddCell()
		if num, err := strconv.Atoi(value); err == nil {
			counts[idx] = num
			switch format[strconv.Itoa(idx)] {
			case "code":
				cell.SetFloatWithFormat(float64(num), "0")
			default:
				cell.SetFloatWithFormat(float64(num), "#,##0")
			}
		} else {
			if empty(value) {
				cell.Value = "Unknown"
			} else {
				cell.Value = value
			}
		}
	}
	return counts, nil
}

func writeHeaderRowToSheet(columns []string, sheet string) error {

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
	for _, col := range columns {
		cell = row.AddCell()
		cell.SetStyle(headerStyle)
		cell.Value = strings.ToTitle(strings.Replace(col, "_", " ", -1))
	}
	return nil
}

func writeFooterRowToSheet(totals []int64, sheet string) error {
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
	for i := 1; i < len(totals); i++ {
		cell = row.AddCell()
		cell.SetStyle(footerStyle)
		cell.SetFloatWithFormat(float64(totals[i]), "#,##0")
	}

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

func sendEmail(username string, password string, release string, file string, layout string) error {
	e := email.NewEmail()
	e.From = "Releases <system@cmsdm.com>"
	e.To = []string{"releases@cmsdm.com"}
	e.Subject = fmt.Sprintf("Data Dictionary was generated for %s.", release)
	e.Text = []byte("Please see attached.")
	e.AttachFile(file)
	if !empty(layout) {
		e.AttachFile(layout)
	}
	err := e.Send("smtp.gmail.com:587", smtp.PlainAuth("", username, password, "smtp.gmail.com"))
	return err
}

func main() {

	layoutFile := flag.String("l", "", "Excel layout file")
	configFile := flag.String("c", "", "Excel out file")
	emailCreds := flag.String("s", "", "Send email with credentials username:password")
	releaseName := flag.String("r", "Release", "Release Name")

	flag.Parse()

	if len(flag.Args()) != 1 {
		log.Fatalln("Usage: ddg -c <config.json> [-l layout.xlsx] <out_dd_name.xlsx>")
	}

	outFile := flag.Args()[0]

	//Load Input File
	file, err := ioutil.ReadFile(*configFile)
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

		fmt.Printf("Getting breakout for: %s...", group.Statement)

		rows, err := db.Query(group.Statement)
		if err != nil {
			log.Fatalln(err.Error())
		}

		cols, err := rows.Columns()
		if err != nil {
			log.Fatalln(err.Error())
		}

		totals := make([]int64, len(cols))

		err = writeHeaderRowToSheet(cols, group.Sheet)
		if err != nil {
			log.Fatalln(err.Error())
		}
		defer rows.Close()

		for rows.Next() {

			data := make([]interface{}, len(cols))
			values := make([][]byte, len(cols))
			for i := range values {
				data[i] = &values[i]
			}

			if err := rows.Scan(data...); err != nil {
				fmt.Println(err)
			}

			counts, err := writeRowToSheet(values, group.Sheet, group.Format)
			if err != nil {
				log.Fatalln(err.Error())
			}

			for i, count := range counts {
				totals[i] += int64(count)
			}

		}
		if err := rows.Err(); err != nil {
			log.Fatalln(err.Error())
		}

		if group.Group {
			writeFooterRowToSheet(totals, group.Sheet)
		}

		if len(group.Mappings) > 0 {
			writeMappingsToSheet(group.Mappings, group.Sheet)
		}

		fmt.Println("done")

	}

	// Format sheets to show complete values
	for _, sheet := range dd.Sheets {
		sheet.SetColWidth(0, len(sheet.Cols)-1, 18)
	}

	// if len(*layoutFile) > 0 {
	// 	lFile, err := xlsx.OpenFile(*layoutFile)
	// 	if err != nil {
	// 		log.Fatalln(err.Error())
	// 	}
	//
	// 	layoutSheet, err := dd.AddSheet("Layout")
	// 	if err != nil {
	// 		log.Fatalln(err.Error())
	// 	}
	//
	// 	oldSheet := lFile.Sheets[0]
	//
	// 	if len(lFile.Sheets) > 0 {
	// 		for _, row := range oldSheet.Rows {
	// 			nRow := layoutSheet.AddRow()
	// 			for _, cell := range row.Cells {
	// 				nCell := nRow.AddCell()
	// 				nCell.SetValue(cell.Value)
	// 				nCell.SetStyle(cell.GetStyle())
	// 			}
	// 		}
	//
	// 		for i, col := range oldSheet.Cols {
	// 			layoutSheet.Col(i).SetStyle(col.GetStyle())
	// 			layoutSheet.Col(i).Width = col.Width
	// 		}
	// 	}
	//
	// }

	// Export excel file
	err = dd.Save(outFile)
	if err != nil {
		log.Fatalln(err.Error())
	}

	//Send email
	if len(*emailCreds) > 0 {
		creds := strings.Split(*emailCreds, ":")

		err := sendEmail(creds[0], creds[1], *releaseName, outFile, *layoutFile)
		if err != nil {
			log.Println(err.Error())
		}
	}
}

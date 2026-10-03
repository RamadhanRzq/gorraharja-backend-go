package pdf

import (
	"bytes"
	"fmt"
	"time"

	"booking-manager/internal/format"
	"booking-manager/internal/model"

	"github.com/go-pdf/fpdf"
)

// Rekap membangun PDF rekap booking di memori.
// start/cutoff adalah periode inklusif; printedAt dicetak dalam WIB.
func Rekap(items []model.Booking, start, cutoff, printedAt time.Time) ([]byte, error) {
	wib := printedAt.In(time.FixedZone("WIB", 7*3600))

	doc := fpdf.New("L", "mm", "A4", "")
	doc.SetTitle("Rekap Booking", false)
	doc.SetAuthor("Booking Manager", false)
	doc.AliasNbPages("{nb}")
	doc.SetAutoPageBreak(true, 15)

	const pageWidth = 297.0

	widths := []float64{12, 30, 30, 22, 95, 45}
	headers := []string{"No", "Tanggal", "Jam", "Durasi", "Nama Penyewa", "Nominal"}

	var tableWidth float64
	for _, width := range widths {
		tableWidth += width
	}

	// Center tabel secara horizontal pada A4 landscape.
	tableX := (pageWidth - tableWidth) / 2

	drawHeader := func() {
		doc.SetX(tableX)
		doc.SetFont("Arial", "B", 10)
		doc.SetFillColor(230, 230, 230)

		for i, header := range headers {
			doc.CellFormat(
				widths[i],
				8,
				header,
				"1",
				0,
				"C",
				true,
				0,
				"",
			)
		}

		doc.Ln(-1)
	}

	doc.AddPage()

	// Title
	doc.SetFont("Arial", "B", 16)
	doc.CellFormat(
		0,
		10,
		"Rekap Booking",
		"",
		1,
		"C",
		false,
		0,
		"",
	)

	// Periode
	doc.SetFont("Arial", "", 11)
	doc.CellFormat(
		0,
		7,
		fmt.Sprintf(
			"Periode: %s s/d %s",
			format.TanggalID(start),
			format.TanggalID(cutoff),
		),
		"",
		1,
		"C",
		false,
		0,
		"",
	)

	// Waktu cetak
	doc.CellFormat(
		0,
		7,
		fmt.Sprintf(
			"Dicetak: %s WIB",
			wib.Format("02 Jan 2006 15:04"),
		),
		"",
		1,
		"C",
		false,
		0,
		"",
	)

	doc.Ln(4)

	// Header tabel
	drawHeader()

	doc.SetFont("Arial", "", 10)

	var total int64

	if len(items) == 0 {
		doc.SetX(tableX)
		doc.CellFormat(
			tableWidth,
			9,
			"Tidak ada data pada periode ini",
			"1",
			1,
			"C",
			false,
			0,
			"",
		)
	} else {
		for i, booking := range items {
			// Header tabel diulang ketika membuat halaman baru.
			if doc.GetY()+9 > 190 {
				doc.AddPage()
				drawHeader()
				doc.SetFont("Arial", "", 10)
			}

			total += booking.NominalPembayaran

			row := []string{
				fmt.Sprintf("%d", i+1),
				format.TanggalID(booking.Tanggal),
				format.RentangJam(booking.Jam, booking.Durasi),
				format.DurasiLabel(booking.Durasi),
				booking.NamaPenyewa,
				format.Rupiah(booking.NominalPembayaran),
			}

			aligns := []string{"C", "C", "C", "C", "L", "R"}

			doc.SetX(tableX)

			for j, value := range row {
				doc.CellFormat(
					widths[j],
					8,
					value,
					"1",
					0,
					aligns[j],
					false,
					0,
					"",
				)
			}

			doc.Ln(-1)
		}
	}

	// Summary
	doc.SetFont("Arial", "B", 10)
	doc.SetFillColor(245, 245, 245)

	labelWidth := tableWidth - widths[5]

	doc.SetX(tableX)

	doc.CellFormat(
		labelWidth,
		8,
		fmt.Sprintf("Jumlah booking: %d", len(items)),
		"1",
		0,
		"R",
		true,
		0,
		"",
	)

	doc.CellFormat(
		widths[5],
		8,
		format.Rupiah(total),
		"1",
		0,
		"R",
		true,
		0,
		"",
	)

	doc.Ln(-1)

	var buf bytes.Buffer

	if err := doc.Output(&buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
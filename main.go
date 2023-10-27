package main

import (
	"bytes"
	"database/sql"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "bookdb.sqlite")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	createTableSQL := `
    CREATE TABLE IF NOT EXISTS books (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        title TEXT,
        author TEXT,
        year INTEGER,
        publisher TEXT,
        copies INTEGER,
        cover BLOB
    );
    `
	_, err = db.Exec(createTableSQL)
	if err != nil {
		log.Fatal(err)
	}

	r := mux.NewRouter()
	r.HandleFunc("/", ListBooks).Methods("GET")
	r.HandleFunc("/", AddBook).Methods("POST")
	r.HandleFunc("/delete/{id:[0-9]+}", DeleteBook).Methods("GET")
	r.HandleFunc("/cover/{id:[0-9]+}", ShowCover).Methods("GET")
	r.HandleFunc("/cover/{id:[0-9]+}", UploadCover).Methods("POST")
	r.HandleFunc("/edit/{id:[0-9]+}", EditBook).Methods("GET")
	r.HandleFunc("/edit/{id:[0-9]+}", UpdateBook).Methods("POST")
	r.HandleFunc("/add", AddBookPage).Methods("GET") // New route for the "Add Book" page

	http.Handle("/", r)

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func ListBooks(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "bookdb.sqlite")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT * FROM books ORDER BY title")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var books []Book
	for rows.Next() {
		var book Book
		err := rows.Scan(&book.ID, &book.Title, &book.Author, &book.Year, &book.Publisher, &book.Copies, &book.Cover)
		if err != nil {
			log.Fatal(err)
		}
		books = append(books, book)
	}

	tmpl, err := template.New("index").Parse(htmlTemplate)
	if err != nil {
		log.Fatal(err)
	}

	data := struct {
		Books []Book
	}{Books: books}

	tmpl.Execute(w, data)
}

func AddBook(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "bookdb.sqlite")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	title := r.FormValue("title")
	author := r.FormValue("author")
	year, _ := strconv.Atoi(r.FormValue("year"))
	publisher := r.FormValue("publisher")
	copies, _ := strconv.Atoi(r.FormValue("copies"))

	cover, _, err := r.FormFile("cover")
	if err != nil {
		log.Print(err)
	}

	coverData, err := readImageFile(cover)
	if err != nil {
		log.Fatal(err)
	}

	insertSQL := "INSERT INTO books (title, author, year, publisher, copies, cover) VALUES (?, ?, ?, ?, ?, ?)"
	_, err = db.Exec(insertSQL, title, author, year, publisher, copies, coverData)
	if err != nil {
		log.Fatal(err)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func DeleteBook(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "bookdb.sqlite")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	deleteSQL := "DELETE FROM books WHERE id = ?"
	_, err = db.Exec(deleteSQL, id)
	if err != nil {
		log.Fatal(err)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func ShowCover(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "bookdb.sqlite")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	var cover []byte
	err = db.QueryRow("SELECT cover FROM books WHERE id = ?", id).Scan(&cover)
	if err != nil {
		log.Fatal(err)
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Write(cover)
}

func UploadCover(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "bookdb.sqlite")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	cover, _, err := r.FormFile("cover")
	if err != nil {
		log.Print(err)
	}

	coverData, err := readImageFile(cover)
	if err != nil {
		log.Fatal(err)
	}

	updateSQL := "UPDATE books SET cover = ? WHERE id = ?"
	_, err = db.Exec(updateSQL, coverData, id)
	if err != nil {
		log.Fatal(err)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func EditBook(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "bookdb.sqlite")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	var book Book
	err = db.QueryRow("SELECT * FROM books WHERE id = ?", id).Scan(&book.ID, &book.Title, &book.Author, &book.Year, &book.Publisher, &book.Copies, &book.Cover)
	if err != nil {
		log.Fatal(err)
	}

	// Set the default "Keep Existing Cover" option
	book.KeepExistingCover = true

	tmpl, err := template.New("edit").Parse(editTemplate)
	if err != nil {
		log.Fatal(err)
	}

	tmpl.Execute(w, book)
}

func UpdateBook(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "bookdb.sqlite")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	id, _ := strconv.Atoi(r.FormValue("id"))
	title := r.FormValue("title")
	author := r.FormValue("author")
	year, _ := strconv.Atoi(r.FormValue("year"))
	publisher := r.FormValue("publisher")
	copies, _ := strconv.Atoi(r.FormValue("copies"))

	// Check if a new cover file has been uploaded
	newCover, _, err := r.FormFile("cover")
	if err != nil {
		log.Print(err)
	}

	// Initialize the coverData variable
	var coverData []byte

	// Check if a new cover is uploaded
	if newCover != nil {
		coverData, err = readImageFile(newCover)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		// If no new cover is uploaded, check if "keep_cover" is checked
		if r.FormValue("keep_cover") == "on" {
			// "Keep Existing Cover" is checked, so retrieve the existing cover
			var existingCover []byte
			err = db.QueryRow("SELECT cover FROM books WHERE id = ?", id).Scan(&existingCover)
			if err != nil {
				log.Fatal(err)
			}
			coverData = existingCover
		}
	}

	updateSQL := "UPDATE books SET title = ?, author = ?, year = ?, publisher = ?, copies = ?, cover = ? WHERE id = ?"
	_, err = db.Exec(updateSQL, title, author, year, publisher, copies, coverData, id)
	if err != nil {
		log.Fatal(err)
	}

	// Scroll to the updated book in book list
	http.Redirect(w, r, "/#book-"+strconv.Itoa(id), http.StatusSeeOther)
}

func readImageFile(file multipart.File) ([]byte, error) {
	var buf bytes.Buffer
	_, err := io.Copy(&buf, file)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func AddBookPage(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.New("addbook").Parse(addBookTemplate)
	if err != nil {
		log.Fatal(err)
	}
	tmpl.Execute(w, nil)
}

type Book struct {
	ID                int
	Title             string
	Author            string
	Year              int
	Publisher         string
	Copies            int
	Cover             []byte
	KeepExistingCover bool // Add this field
}

const htmlTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Bookshop</title>
    <style>
        /* Custom CSS for the fixed header and search bar */
        .fixed-header {
            position: fixed;
            top: 0;
            left: 0;
            padding-left: 20px;
            background: black;
            color: #fff;
            width: 100%;
            z-index: 1;
            display: flex;
            align-items: center;
        }

        .search-bar {
            padding: 10px;
            margin: 10px 10px 10px 0;
            width: 300px;
        }

        .search-button {
            padding: 10px;
            background: black;
            color: #fff;
            border: none;
            cursor: pointer;
            transition: border 0.3s;
        }

        /* Add a border on hover */
        .search-button:hover {
            border: 2px solid white;
        }

        .add-button {
            padding: 10px;
            background: black;
            color: #fff;
            border: none;
            cursor: pointer;
            transition: border 0.3s;
        }

        .add-button:hover {
            border: 2px solid white;
        }

        .title-counter {
            padding: 10px;
            color: #fff;
        }

        .total-copies {
            padding: 10px;
            color: #fff;
        }

        .book-list-container {
            margin-top: 50px;
            overflow-y: auto;
            max-height: calc(100vh - 130px);
        }

        .book-list table {
            width: 100%;
            margin-top: 20px;
        }

        .book-list th {
            width: 50px;
            padding-left: 10px;
        }

        .book-list td {
            text-align: left;
            padding: 5px;
        }

        .book-cover {
            display: block;
            margin: 0 auto;
            padding-left: 10px;
            padding-right: 10px;
            cursor: pointer;
        }

        /* Updated CSS for the footer */
        footer {
            position: fixed;
            bottom: 0;
            left: 0;
            background: black;
            color: #fff;
            width: 100%;
            padding: 10px;
            text-align: center;
        }

        .image-overlay {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background-color: rgba(0, 0, 0, 0.9);
            text-align: center;
            z-index: 2;
        }

        .image-overlay img {
            max-width: 80%;
            max-height: 80%;
            position: absolute;
            top: 50%;
            left: 50%;
            transform: translate(-50%, -50%);
        }

        .close-button {
            color: #fff;
            font-size: 20px;
            position: absolute;
            top: 10px;
            right: 20px;
            cursor: pointer;
        }
    </style>
</head>
<body>
    <!-- Create the fixed header with "Add Book" button and search bar -->
    <div class="fixed-header">
        <input type="text" id="searchTitle" class="search-bar" placeholder="Search Book Title">
        <button onclick="searchBook()" class="search-button">Search</button>
        <button onclick="window.location.href='/add'" class="add-button">Add Book</button>
        <div class="title-counter" id="titleCounter">Titles: 0</div>
        <div class="total-copies" id="totalCopies">Books: 0</div>
    </div>

    <div class="book-list-container">
        <table class="book-list">
            <thead>
                <tr>
                    <th></th>
                </tr>
            </thead>
            <tbody>
                {{range .Books}}
                <tr class="book" id="book-{{.ID}}">
                    <td>
                        <a href="javascript:void(0);" onclick="showCover('{{.ID}}')">
                            <img src="/cover/{{.ID}}" alt="{{.Title}} Cover" width="100" class="book-cover">
                        </a>
                    </td>
                    <td class="book-details">
                        <div class="book-title">Title: {{.Title}}</div>
                        <div>Author: {{.Author}}</div>
                        <div>Year: {{.Year}}</div>
                        <div>Publisher: {{.Publisher}}</div>
                        <div>Copies: {{.Copies}}</div>
                        <div>
                            <a href="/delete/{{.ID}}">Delete</a>
                            <a href="/edit/{{.ID}}">Edit</a>
                        </div>
                    </td>
                </tr>
                {{end}}
            </tbody>
        </table>
    </div>

    <!-- Image overlay for larger images -->
    <div class="image-overlay" id="imageOverlay">
        <span class="close-button" onclick="closeCover()">&times;</span>
        <img id="expandedCover">
    </div>

    <!-- Footer -->
    <footer>
        Developed for - The Bookshop
    </footer>

    <script>
        var currentBookIndex = -1;
        var searchTitle = "";

        function searchBook() {
            searchTitle = document.getElementById("searchTitle").value.toLowerCase();
            var bookTitles = document.querySelectorAll(".book-title");

            var found = false;

            for (var i = currentBookIndex + 1; i < bookTitles.length; i++) {
                var bookTitle = bookTitles[i].textContent.toLowerCase();

                if (bookTitle.includes(searchTitle)) {
                    var bookElement = bookTitles[i].closest("tr");
                    if (bookElement) {
                        bookElement.style.display = "table-row";
                        currentBookIndex = i;
                        document.querySelector(".book-list-container").scrollTop = bookElement.offsetTop;
                        found = true;
                        break;
                    }
                }
            }

            if (!found) {
                currentBookIndex = -1;
                alert("No more matches found.");
            }
        }

        function showCover(bookId) {
            var cover = document.getElementById("expandedCover");
            cover.src = "/cover/" + bookId;
            var overlay = document.getElementById("imageOverlay");
            overlay.style.display = "block";
        }

        function closeCover() {
            var overlay = document.getElementById("imageOverlay");
            overlay.style.display = "none";
        }

        window.onclick = function(event) {
            var overlay = document.getElementById("imageOverlay");
            if (event.target == overlay) {
                overlay.style.display = "none";
            }
        }

        // Function to count titles with copies > 0
        function countTitlesWithCopies() {
            var bookCopies = document.querySelectorAll(".book-details > div:nth-child(5)");
            var count = 0;
            var totalCopies = 0;

            bookCopies.forEach(function (element) {
                var copies = parseInt(element.textContent.replace("Copies: ", ""));
                if (copies > 0) {
                    count++;
                    totalCopies += copies;
                }
            });

            var titleCounter = document.getElementById("titleCounter");
            titleCounter.textContent = "Titles: " + count;

            var totalCopiesElement = document.getElementById("totalCopies");
            totalCopiesElement.textContent = "Books: " + totalCopies;
        }

        // Call the countTitlesWithCopies function on page load
        countTitlesWithCopies();
    </script>
</body>
</html>
`

const editTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Edit Book</title>
</head>
<body>
    <h1>Edit Book</h1>
    <form method="POST" enctype="multipart/form-data">
        <input type="hidden" name="id" value="{{.ID}}">
        <label for="title">Title:</label>
        <input type="text" id="title" name="title" value="{{.Title}}" required>
        <br>
        <label for="author">Author:</label>
        <input type="text" id="author" name="author" value="{{.Author}}" required>
        <br>
        <label for="year">Year:</label>
        <input type="number" id="year" name="year" value="{{.Year}}" required>
        <br>
        <label for="publisher">Publisher:</label>
        <input type="text" id="publisher" name="publisher" value="{{.Publisher}}" required>
        <br>
        <label for="copies">Copies Available:</label>
        <input type="number" id="copies" name="copies" value="{{.Copies}}" required>
        <br>
        <label for="cover">ONLY Use to Change Existing Cover Image:</label>
        <input type="file" name="cover" accept="image/*">
        <br>
        <!-- Hide the "Keep Existing Cover" checkbox with inline CSS -->
        <label for="keep_cover" style="display: none;">Keep Existing Cover:</label>
        <input type="checkbox" name="keep_cover" style="display: none;" checked>
        <br>
        <button type="submit">Update Book</button>
    </form>
</body>
</html>
`

const addBookTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Add Book</title>
</head>
<body>
    <h1>Add Book</h1>
    <form method="POST" action="/" enctype="multipart/form-data">
        <!-- Add the input fields for adding a book -->
        <label for="title">Title:</label>
        <input type="text" id="title" name="title" required>
        <br>
        <label for="author">Author:</label>
        <input type="text" id="author" name="author" required>
        <br>
        <label for="year">Year:</label>
        <input type="number" id="year" name="year" required>
        <br>
        <label for="publisher">Publisher:</label>
        <input type="text" id="publisher" name="publisher" required>
        <br>
        <label for="copies">Copies Available:</label>
        <input type="number" id="copies" name="copies" required>
        <br>
        <label for="cover">Cover Image:</label>
        <input type="file" name="cover" accept="image/*">
        <br>
        <button type="submit">Add Book</button>
    </form>
</body>
</html>
`

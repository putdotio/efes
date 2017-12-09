package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func (t *Tracker) iterFiles(w http.ResponseWriter, r *http.Request) {
	var err error
	from := uint64(0)
	count := uint64(1000)

	fromStr := r.FormValue("from")
	if fromStr != "" {
		from, err = strconv.ParseUint(fromStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid param: from", http.StatusBadRequest)
			return
		}
	}
	countStr := r.FormValue("count")
	if countStr != "" {
		count, err = strconv.ParseUint(countStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid param: count", http.StatusBadRequest)
			return
		}
	}

	type file struct {
		ID  int64  `json:"id"`
		Key string `json:"key"`
	}
	files := make([]file, 0)
	rows, err := t.db.Query("select fid, dkey from file where fid > ? limit ?", from, count)
	if err != nil {
		t.internalServerError("cannot get keys from database", err, r, w)
		return
	}
	defer rows.Close() // nolint: errcheck
	for rows.Next() {
		var f file
		err = rows.Scan(&f.ID, &f.Key)
		if err != nil {
			t.internalServerError("cannot scan row", err, r, w)
			return
		}
		files = append(files, f)
	}
	err = rows.Err()
	if err != nil {
		t.internalServerError("cannot close rows", err, r, w)
		return
	}
	response := struct {
		Files []file `json:"files"`
	}{
		Files: files,
	}
	encoder := json.NewEncoder(w)
	encoder.Encode(response) // nolint: errcheck
}

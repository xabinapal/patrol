package cli

import (
	"bytes"
	"testing"
)

func TestParseOutputFormat(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    OutputFormat
		wantErr bool
	}{
		{
			name:    "text format",
			input:   "text",
			want:    OutputFormatText,
			wantErr: false,
		},
		{
			name:    "json format",
			input:   "json",
			want:    OutputFormatJSON,
			wantErr: false,
		},
		{
			name:    "empty string defaults to text",
			input:   "",
			want:    OutputFormatText,
			wantErr: false,
		},
		{
			name:    "invalid format",
			input:   "xml",
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid format yaml",
			input:   "yaml",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseOutputFormat(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseOutputFormat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseOutputFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOutputWriter_IsJSON(t *testing.T) {
	tests := []struct {
		name   string
		format OutputFormat
		want   bool
	}{
		{
			name:   "json format returns true",
			format: OutputFormatJSON,
			want:   true,
		},
		{
			name:   "text format returns false",
			format: OutputFormatText,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := NewOutputWriter(tt.format)
			if got := o.IsJSON(); got != tt.want {
				t.Errorf("OutputWriter.IsJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOutputWriter_WriteJSON(t *testing.T) {
	type testData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	data := testData{Name: "test", Value: 42}

	var buf bytes.Buffer
	o := &OutputWriter{
		format: OutputFormatJSON,
		writer: &buf,
	}

	err := o.WriteJSON(data)
	if err != nil {
		t.Errorf("WriteJSON() error = %v", err)
		return
	}

	expected := "{\n  \"name\": \"test\",\n  \"value\": 42\n}\n"
	if got := buf.String(); got != expected {
		t.Errorf("WriteJSON() output = %q, want %q", got, expected)
	}
}

func TestOutputWriter_Write(t *testing.T) {
	type testData struct {
		Name string `json:"name"`
	}

	t.Run("json format writes JSON", func(t *testing.T) {
		var buf bytes.Buffer
		o := &OutputWriter{
			format: OutputFormatJSON,
			writer: &buf,
		}

		data := testData{Name: "test"}
		textCalled := false
		err := o.Write(data, func() {
			textCalled = true
		})

		if err != nil {
			t.Errorf("Write() error = %v", err)
		}
		if textCalled {
			t.Error("Write() called textFunc when format is JSON")
		}
		if buf.Len() == 0 {
			t.Error("Write() did not write JSON output")
		}
	})

	t.Run("text format calls textFunc", func(t *testing.T) {
		var buf bytes.Buffer
		o := &OutputWriter{
			format: OutputFormatText,
			writer: &buf,
		}

		data := testData{Name: "test"}
		textCalled := false
		err := o.Write(data, func() {
			textCalled = true
		})

		if err != nil {
			t.Errorf("Write() error = %v", err)
		}
		if !textCalled {
			t.Error("Write() did not call textFunc when format is text")
		}
	})
}

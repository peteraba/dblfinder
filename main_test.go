package main

import (
	"reflect"
	"testing"
)

func Test_parseRead(t *testing.T) {
	type args struct {
		s   string
		max int
	}
	tests := []struct {
		name string
		args args
		want []int
		ok   bool
	}{
		{
			"one-of-two",
			args{
				"1",
				2,
			},
			[]int{1},
			true,
		},
		{
			"two-three-four-of-seven",
			args{
				"2 3 4",
				7,
			},
			[]int{2, 3, 4},
			true,
		},
		{
			"two-four-five-of-seven",
			args{
				"2 4 3",
				7,
			},
			[]int{2, 3, 4},
			true,
		},
		{
			"range-two-to-five-of-seven",
			args{
				"2-5",
				7,
			},
			[]int{2, 3, 4, 5},
			true,
		},
		{
			"range-two-to-five-and-seven-of-seven",
			args{
				"2-5 7",
				7,
			},
			[]int{2, 3, 4, 5, 7},
			true,
		},
		{
			"range-two-to-five-and-seven-of-seven",
			args{
				"7 2-5 3",
				7,
			},
			[]int{2, 3, 4, 5, 7},
			true,
		},
		{
			"empty-of-two",
			args{
				"",
				2,
			},
			nil,
			false,
		},
		{
			"range-zero-to-five-of-seven",
			args{
				"0-5",
				7,
			},
			nil,
			false,
		},
		{
			"range-five-to-four-of-seven",
			args{
				"5-4",
				7,
			},
			nil,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseRead(tt.args.s, tt.args.max)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseRead() got = %v, want %v", got, tt.want)
			}
			if ok != tt.ok {
				t.Errorf("parseRead() ok = %v, want %v", ok, tt.ok)
			}
		})
	}
}

func Test_uniqueInts(t *testing.T) {
	type args struct {
		ints []int
	}
	tests := []struct {
		name string
		args args
		want []int
	}{
		{
			"one-one-two-three-two",
			args{
				[]int{1, 1, 2, 3, 2},
			},
			[]int{1, 2, 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := uniqueInts(tt.args.ints); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("uniqueInts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_uniqueStrings(t *testing.T) {
	type args struct {
		arr []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			"one-one-two-three-two",
			args{
				[]string{"one", "one", "two", "three", "two"},
			},
			[]string{"one", "three", "two"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := uniqueStrings(tt.args.arr); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("uniqueStrings() = %v, want %v", got, tt.want)
			}
		})
	}
}
package main

import "testing"

func TestGenerateBrokerPairs(t *testing.T) {
	cases := []struct {
		testDescription string
		input           []string
		output          []brokerPair
		expectedError   string
	}{
		{
			testDescription: "empty input",
			expectedError:   "received 0 item(s) in list but at least 2 are required",
		},
		{
			testDescription: "one string",
			input:           []string{"a"},
			expectedError:   "received 1 item(s) in list but at least 2 are required",
		},
		{
			testDescription: "two strings",
			input:           []string{"a", "b"},
			output: []brokerPair{
				{
					source:      "a",
					destination: "b",
				},
				{
					source:      "b",
					destination: "a",
				},
			},
		},
		{
			testDescription: "three strings",
			input:           []string{"a", "b", "c"},
			output: []brokerPair{
				{
					source:      "a",
					destination: "b",
				},
				{
					source:      "a",
					destination: "c",
				},
				{
					source:      "b",
					destination: "a",
				},
				{
					source:      "b",
					destination: "c",
				},
				{
					source:      "c",
					destination: "a",
				},
				{
					source:      "c",
					destination: "b",
				},
			},
		},
		{
			testDescription: "four strings",
			input:           []string{"a", "b", "c", "d"},
			output: []brokerPair{
				{
					source:      "a",
					destination: "b",
				},
				{
					source:      "a",
					destination: "c",
				},
				{
					source:      "a",
					destination: "d",
				},
				{
					source:      "b",
					destination: "a",
				},
				{
					source:      "b",
					destination: "c",
				},
				{
					source:      "b",
					destination: "d",
				},
				{
					source:      "c",
					destination: "a",
				},
				{
					source:      "c",
					destination: "b",
				},
				{
					source:      "c",
					destination: "d",
				},
				{
					source:      "d",
					destination: "a",
				},
				{
					source:      "d",
					destination: "b",
				},
				{
					source:      "d",
					destination: "c",
				},
			},
		},
	}

	for i, c := range cases {
		t.Logf("Test #%d: %s", i, c.testDescription)
		result, err := generateBrokerPairs(c.input, "mqtt-pinger")
		testError(t, err, c.expectedError)

		if len(c.output) != len(result) {
			t.Fatalf("Expected length %d got length %d", len(c.output), len(result))
		}
		for i := range result {
			if c.output[i].source != result[i].source || c.output[i].destination != result[i].destination {
				t.Fatalf("\ngot:\t%s\nwant:\t%s\n", result, c.output)
			}
		}
	}
}

func testError(t *testing.T, err error, expected string) {
	t.Helper()

	if err == nil && expected != "" {
		t.Fatalf("expected error %q but received none", expected)
	}

	if err != nil && expected == "" {
		t.Fatalf("expected no error but received %q", err)
	}

	if err != nil && err.Error() != expected {
		t.Fatalf("expected %q but received %q", expected, err)
	}
}

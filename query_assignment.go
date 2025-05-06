package main

type queryAssignmentData struct {
	Set   []string
	Unset []string
}

type queryAssignment struct {
	Query      map[string]string   `json:"query"`
	Categories queryAssignmentData `json:"categories"`
	Tags       queryAssignmentData `json:"tags"`
}

type queryAssignments struct {
	RepeatSecs  int               `json:"repeat-secs"`
	TimeoutSecs int               `json:"timeout-secs"`
	Assignments []queryAssignment `json:"assignments"`
}

// // An example JSON config follows:
// {
//     "repeat-secs": 30,
//     "timeout-secs": 30,
//     "assignments": [
//         {
//             "query": {
//                 "queryFilter": "lastMade IS NOT NULL"
//             },
//             "categories": {
//                 "set": ["Made"],
//                 "unset": ["NotMade"]
//             },
//             "tags": {
//                 "set": ["Yummy", "Unknown"],
//                 "unset": []
//             }
//         },
//         {
//             "query": {
//                 "queryFilter": "lastMade IS NULL"
//             },
//             "categories": {
//                 "set": ["NotMade"],
//                 "unset": ["Made"]
//             },
//             "tags": {
//                 "set": ["Unknown"],
//                 "unset": ["Yummy"]
//             }
//         }
//     ]
// }

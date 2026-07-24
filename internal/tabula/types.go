// Package tabula resolves TABULA building defaults from ignis when a BuEM
// request omits building.envelope, and maps them into BuEM's envelope/thermal
// shape. Only the fields ignis's GET /api/v1/data/:code actually populates
// are modeled here — that endpoint returns raw TABULA workbook values, not
// ignis's own computed pipeline outputs (phi_int, c_m, shading factors,
// per-element b_transmission/g_gl, ...), which come back null. Those are left
// for BuEM to default, the same way they're defaulted for any other request.
package tabula

// variantMatch is one entry in ignis's GET /api/v1/variants/:country/match response.
type variantMatch struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

// matchResponse is ignis's GET /api/v1/variants/:country/match response.
type matchResponse struct {
	Data []variantMatch `json:"data"`
}

// dataResponse is ignis's GET /api/v1/data/:code response.
type dataResponse struct {
	TabulaData struct {
		BasicParameters struct {
			BuildingAppearance struct {
				NStorey         int     `json:"n_Storey"`
				HRoom           float64 `json:"h_room"`
				AttachedNeighb  string  `json:"Code_AttachedNeighbours"`
				AtticCondition  string  `json:"Code_AtticCond"`
				CellarCondition string  `json:"Code_CellarCond"`
			} `json:"BuildingAppearance"`
			Envelope struct {
				ACRefEstim float64 `json:"A_C_Ref_Estim"`
				ACLiving   float64 `json:"A_C_Living"`

				ARoof1  float64 `json:"A_Roof_1"`
				ARoof2  float64 `json:"A_Roof_2"`
				AWall1  float64 `json:"A_Wall_1"`
				AWall2  float64 `json:"A_Wall_2"`
				AWall3  float64 `json:"A_Wall_3"`
				AFloor1 float64 `json:"A_Floor_1"`
				AFloor2 float64 `json:"A_Floor_2"`

				AWindow1          float64 `json:"A_Window_1"`
				AWindow2          float64 `json:"A_Window_2"`
				AWindowHorizontal float64 `json:"A_Window_Horizontal"`
				AWindowEast       float64 `json:"A_Window_East"`
				AWindowSouth      float64 `json:"A_Window_South"`
				AWindowWest       float64 `json:"A_Window_West"`
				AWindowNorth      float64 `json:"A_Window_North"`

				ADoor1 float64 `json:"A_Door_1"`
			} `json:"Envelope"`
		} `json:"BasicParameters"`
		AdvancedParameters struct {
			AirInfiltration struct {
				NAirInfiltration float64 `json:"n_air_infiltration"`
				NAirUse          float64 `json:"n_air_use"`
			} `json:"AirInfiltration"`
			Uvalues struct {
				URoof1   float64 `json:"U_Roof_1"`
				URoof2   float64 `json:"U_Roof_2"`
				UWall1   float64 `json:"U_Wall_1"`
				UWall2   float64 `json:"U_Wall_2"`
				UWall3   float64 `json:"U_Wall_3"`
				UFloor1  float64 `json:"U_Floor_1"`
				UFloor2  float64 `json:"U_Floor_2"`
				UWindow1 float64 `json:"U_Window_1"`
				UWindow2 float64 `json:"U_Window_2"`
				UDoor1   float64 `json:"U_Door_1"`
			} `json:"Uvalues"`
		} `json:"AdvancedParameters"`
	} `json:"tabula_data"`
}

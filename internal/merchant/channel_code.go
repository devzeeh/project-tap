package merchant

// channelCodeMap maps a merchant's selected settlement bank/e-wallet name
// (as stored in the DB / shown in the UI) to Xendit's payout channel code.
//
// Source of truth: https://docs.xendit.co/docs/payouts-philippines
// Verify against GetPayoutChannels() periodically — Xendit can add/deprecate
// channels, and this map will silently go stale otherwise.
var channelCodeMap = map[string]string{
	"Asia United Bank (AUB)":                          "PH_AUB",
	"BDO Network Bank":                                "PH_ONB",
	"BDO Unibank":                                     "PH_BDO",
	"Bank of the Philippine Islands (BPI)":            "PH_BPI",
	"CIMB Bank Philippines Inc":                       "PH_CIMB",
	"Development Bank of the Philippines":             "PH_DBP",
	"East West Banking Corporation":                   "PH_EWB",
	"East West RURAL BANK OR KOMO":                    "PH_EWR",
	"GoTyme Bank":                                     "PH_GOTYME",
	"Land Bank of the Philippines":                    "PH_LBP",
	"Maya Bank, Inc.":                                 "PH_MAYA",
	"Metropolitan Bank and Trust Company (Metrobank)": "PH_MET",
	"Philippine National Bank (PNB)":                  "PH_PNB",
	"Philippine Savings Bank (PSBANK)":                "PH_PSB",
	"Seabank Philippines, Inc.":                       "PH_SEA",
	"Security Bank Corporation":                       "PH_SEC",
	"Union Bank of the Philippines (UBP)":             "PH_UBP",
	"Union Digital Bank":                              "PH_UDP",
	"GCash":                                           "PH_GCASH",
	"GrabPay":                                         "PH_GRABPAY",
	"PayMaya":                                         "PH_PAYMAYA",
	"ShopeePay":                                       "PH_SHOPEE",
}
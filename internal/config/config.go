package serviceconfig

const CacheName = "default-cache"
const MaxManualFeedRatePerMinute = 60

type AutoFeederConfig struct {
	Cost            int
	IncomePerMinute int
}

var AutoFeeders = map[int]*AutoFeederConfig{
	1: {
		Cost:            100,
		IncomePerMinute: 5,
	},
	2: {
		Cost:            10000,
		IncomePerMinute: 50,
	},
	3: {
		Cost:            1000000,
		IncomePerMinute: 500,
	},
}

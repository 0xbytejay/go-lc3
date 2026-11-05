package lc3

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
)

// Digests were generated from scalar liblc3 48bbd3eacd36e99a57317a0a4867002e0b09e183
// with reassociation and floating-point contraction disabled. They cover every
// duration/rate pair, resampling, persistent MDCT state, pitch analysis, and
// active/inactive/changing-pitch postfilter transitions.
var transformOracleCases = []struct {
	kind        string
	dt, sr, src int
	sha         string
}{
	{"mdct", 0, 0, 0, "eef3f81229ab5fc421697239465899cba5a8b4dcc33b3b1647ab66af6ece8bbd"},
	{"mdct", 0, 1, 0, "6cd7d8a2ace54c3ac661f90a2deac854d3684ef3f8326c55bbed0202d11d00dc"},
	{"mdct", 0, 1, 1, "ea2a7d645288c2ff262b847c74f0dfb487196a6b72da08ac086a4a606a293db8"},
	{"mdct", 0, 2, 0, "c392ef56365d959a14e412dfc259cf23c465447c9dcd031b36d8638e71f4b741"},
	{"mdct", 0, 2, 1, "f10d71505bda276e753ab210672753e54169a75652e0a4745fd3254d4f877a54"},
	{"mdct", 0, 2, 2, "509cf8b706bc88dddfb69a1a46a5474867ca1b2350956fd52b87c56acf908ee1"},
	{"mdct", 0, 3, 0, "6e20d27a3703a3fccff0108bc428c7393f2003031a05b6dcb7d1c72d47901bb8"},
	{"mdct", 0, 3, 1, "913316e11a37ccf8d66c10e6eba921c28c3d27abad951148dbdcc9eeb0a92c6a"},
	{"mdct", 0, 3, 2, "e3103ce57e7becc6848817b71195b3428f810e8ef17bc9cb1bb6733ed0a03d41"},
	{"mdct", 0, 3, 3, "032410bca10ae78adaca368717dfa1b6f7c2c6e5d6e9b50dd6f011debe275615"},
	{"mdct", 0, 4, 0, "b812889b19cd513c3f7c1784e101576e5e4898ecf9883286a092fa361efb78a5"},
	{"mdct", 0, 4, 1, "04f2974fc5886e80bdeb2ac97a739e406af9623b90b5151fccf4025b16832287"},
	{"mdct", 0, 4, 2, "e559418dbbc6698e0349d9b81b41404d8a5930e89bd72703f36de8d29403301d"},
	{"mdct", 0, 4, 3, "1652238bba3b7052a131e680b1650b7c803aaaa04f9a9421938fea73c6406fff"},
	{"mdct", 0, 4, 4, "addba267573ec8848c851f354f859a7c9c142ec459ce848c62703fc7ac89e9a4"},
	{"mdct", 0, 5, 5, "0da390f4902a4823fb23a9100b8ad47bacb7944b5fafa3c1144833dcb149b1f7"},
	{"mdct", 0, 6, 5, "b7745cc778f277e6de97939f8a135ea60b86ef57f5a5da90ac98d15b26e26fa5"},
	{"mdct", 0, 6, 6, "08bd362b5a4639c8ab82336f24838e124fbe9b35cffc711a18ec28dcf551cf99"},
	{"mdct", 1, 0, 0, "efa8986a51dbe13c34ee354d4e4e9324b750131e8122900807cba95519377745"},
	{"mdct", 1, 1, 0, "b0495ebee38d10415b6dc4f3e8886554877d4f21f61065b91a01a4bb909db6ac"},
	{"mdct", 1, 1, 1, "3ff8161812d0537004a7f62f536518ec92c37ebc95bc01dbb738abbeb5c980c9"},
	{"mdct", 1, 2, 0, "23fa74b3657cdf41dfaad9fd28f81a5d0f4a935dd1ee9be8091578b228e82f15"},
	{"mdct", 1, 2, 1, "8c777bc851a2b9d3c92464ac23e893062696ec857a77ef5653bc7774134d74f1"},
	{"mdct", 1, 2, 2, "aa61913177849f4f41b54edbbab4aacd0bc7610229a8fe534ec0ff369907332c"},
	{"mdct", 1, 3, 0, "96016cdcb77a55c73952c8a094d6bf53952b36da6583e17a5f844c75c97824ce"},
	{"mdct", 1, 3, 1, "f79e6356ac189031e63f493400116690d016029add97edb8a64e6f7942066f5e"},
	{"mdct", 1, 3, 2, "428172067ac6f25abf37e362cbc4c91f809d1d09ffa324c0d3f53215a6231e34"},
	{"mdct", 1, 3, 3, "63b6d5e31de0cd4b0a4dd3e8763f3cbef6d696af2d29c8e9a9651b038bcb0982"},
	{"mdct", 1, 4, 0, "067ee89d64154270e4388cfc2688a95abf59dc428e708d98984031f86a696c81"},
	{"mdct", 1, 4, 1, "a6b0a0fef3e7e2a2e2e5b53b158f744ca4e0ddead967ef02fc386c01a9b43f39"},
	{"mdct", 1, 4, 2, "519d736b839b39688b40b847c5828ffa9ab51a30fe8cb3418bf642c38e204d14"},
	{"mdct", 1, 4, 3, "7ac0e4a2b8452596265e6b87bbf1c5034606175391aa2a1a8c6ebd0ff5f4ac68"},
	{"mdct", 1, 4, 4, "c4b5bb35d95ea8487f8c2a81f029cd31bd80af3f34746e9e40b56b580896487a"},
	{"mdct", 1, 5, 5, "945ef267d16bdf0c591bcea65f644bf3dbbf4a62ef04dcbbe6e7cf9f916a031a"},
	{"mdct", 1, 6, 5, "aece22d855df5678bad3e90053e3a8acd0d2d53944b11f3cd60d7e0b100bebbc"},
	{"mdct", 1, 6, 6, "fb51833a471d7adb73a2249d687b4f8a520e4b56828659d121e8c940747f4284"},
	{"mdct", 2, 0, 0, "041ad00ca975415b5dc3b3d2a30e8bc57571c52ee2afc6f24937de563f425f85"},
	{"mdct", 2, 1, 0, "f500f1f5a5cb80adf7a20902ba75005979415d4d7f866b0da68c89ab5af67b4a"},
	{"mdct", 2, 1, 1, "38d8ffc15f0842bb19b5aa83efcdf26c181526f58e763b9cce5e14db31325baf"},
	{"mdct", 2, 2, 0, "5a496b317071168f4806b0c17fbc18648a672ef8cf764a71ed869290f796c661"},
	{"mdct", 2, 2, 1, "317799c1dba8ab9a237ffe37ae75d30c724c052262a369837fafec6ca17b1766"},
	{"mdct", 2, 2, 2, "0803084eac377a4b1f44ad60e39964be63fe1895ef2b53023bf823a3b8e2a871"},
	{"mdct", 2, 3, 0, "9c0ac354f2307c5184356925c4f6f76d141a2b587b4fe52f64d7582eceb6dfae"},
	{"mdct", 2, 3, 1, "4a8bb48234bdbd698d6449576ce1426616db0c9014629c2eae9cefac5c8d2a96"},
	{"mdct", 2, 3, 2, "86e3268141b356794736286ba811fbd867ba36035b1de72625ff14c90722db4a"},
	{"mdct", 2, 3, 3, "e8296a02b5a42652f5f652f9276473c386bb50c97d8f0139852d88f101370785"},
	{"mdct", 2, 4, 0, "973e42a79eb2976564f7086049e6afc2891e1acc74c8a3d769c8f9afff9c6baf"},
	{"mdct", 2, 4, 1, "f3020a48710d8127f01e66451b230a264c9719d7ea0b564d077101bfe0ef1e0b"},
	{"mdct", 2, 4, 2, "898a2f7faa7e1c2676d8fb2d5a68fe54c62be1f9f104575cf8f94ecb0d55fb2b"},
	{"mdct", 2, 4, 3, "1374793a37ccb727079a0546fbaae3ec960bc08381df7c2beb46ec49b9feac5e"},
	{"mdct", 2, 4, 4, "3943977e80ba574c23f99be3fa5807177602fc23af9f2c935592d36c8b2454f8"},
	{"mdct", 3, 0, 0, "e479a8473ed802c94de32d863f2695916bc4641fe3ac0e9a2a2cdb321cdaa7cf"},
	{"mdct", 3, 1, 0, "027ab9d3ad69ff3df4b01a2709764c84a679962e0a6288659d93fd5abb9625c4"},
	{"mdct", 3, 1, 1, "cab66997d6548108b6046ca20b8aad544c38d6e13c493e2449b84e7127a09287"},
	{"mdct", 3, 2, 0, "6c7f677846c02d694a8f49bb26e96207944e09811a7c23a227eea5b6eca8ad32"},
	{"mdct", 3, 2, 1, "03cda9e5e18ce8f6ec7f5379c9249adc7a7c645b80e7beace8ef21684e30714a"},
	{"mdct", 3, 2, 2, "68423976d8435857da9c2203b6ad17ada8e33aed0645f86d955195869c91aa0a"},
	{"mdct", 3, 3, 0, "cbaf39b792ffe485b0b6be52d3c036c665823554596f1fc4ab72a5fb2d12c048"},
	{"mdct", 3, 3, 1, "7a7a5da8e809963a41f3e37e29eac2f0de4bcbc5c62bfcf816754697585932dd"},
	{"mdct", 3, 3, 2, "d5e3d5e981f51a0a9081414ee0cdd725314e4df7602d4f025d9871baf41ff82c"},
	{"mdct", 3, 3, 3, "d16513ef03fe31cf79776f081244fd3d7af5e465e82b4736c1db59ab33dd115c"},
	{"mdct", 3, 4, 0, "957d6cf6c8551723a37990dd8cb30df1b58a6c168ce4e5fbb8a8b20db2bd48ea"},
	{"mdct", 3, 4, 1, "e75870ff4bd7963196b832e692b4dddb5baa37445152930ff3c1db67270a3a39"},
	{"mdct", 3, 4, 2, "a4b665879a788197441073aecc5bb044a2528c0e10e5a5ffee7771f1ec85d117"},
	{"mdct", 3, 4, 3, "7b7d9fda4961564caeb61bb151f18eca12a5e780b3edd590dadfcf07fecb5f4a"},
	{"mdct", 3, 4, 4, "8e61cd39634979cffcd414e8caf6bd449bef1e087c39c5c527fc390fa88befe9"},
	{"mdct", 3, 5, 5, "e0b70457d2e6267e43f1d9adc2d9be69da68a80889aa5bc848fec0a08248ea6c"},
	{"mdct", 3, 6, 5, "f81fbd6741909f66d20b27fab090e4a6eda5a3d8d8ff4a75e2336945c8ceea99"},
	{"mdct", 3, 6, 6, "6448a609191d776b304180bedca83c96f8a3ab00f38a33259bef753bbb0d96c8"},
	{"ltpf_an", 0, 0, 0, "cab9af921c2c6b3af738b9b7a277cfe27d4474ead1b643e3b4d6abbfaf3cb7dc"},
	{"ltpf_an", 0, 1, 0, "fda4c7e14b0b6c56a9d075dd4600bacd348f94df3b92f0c154034e6f2d5f7fd5"},
	{"ltpf_an", 0, 2, 0, "f2f65abbfc50e54dd3a422fb0914d386c59d07db33191e386594b27fdb75d018"},
	{"ltpf_an", 0, 3, 0, "9c577279c1625d22fba37857c28005377cd17ba4edb8a2c7b377a1c2a9b07281"},
	{"ltpf_an", 0, 4, 0, "161857c64abedff45d33d0f69c64f0a515d04eb5f3d91384a0757f9ae4f19da9"},
	{"ltpf_an", 0, 5, 0, "dca9343e2a66678f52c420e3e5db594d6c5de0a2820000f556407d631222b875"},
	{"ltpf_an", 0, 6, 0, "39b27c984215ccca6eead06e68b5220fe71c34954411bcaca1e9394961c8d9b1"},
	{"ltpf_an", 1, 0, 0, "3e867dbac291479264feedd3b682f44eb569edf9c0bbe49785ddbfcbfe7b3f39"},
	{"ltpf_an", 1, 1, 0, "128b431396edf62337798a2d7555e4490db2dbee4f02e9417e51f6e2df491db4"},
	{"ltpf_an", 1, 2, 0, "ca31bbe00fcfc8493fee8e1b9b3fd6b8f3a79d396eb9dadf4f024bed310df13a"},
	{"ltpf_an", 1, 3, 0, "aee0385db06dcfc9c08421bd154169d20ec75bae7af891a19060a98e34435aeb"},
	{"ltpf_an", 1, 4, 0, "fdb0fe22832acd91bfac52b78bc202e6eeb7fb673a1766bcea92e5c6d7c20f6e"},
	{"ltpf_an", 1, 5, 0, "773e4bf3950044ea15c2949ef7f413d6d768031c5bf91ee9c34bd7794d14cfa9"},
	{"ltpf_an", 1, 6, 0, "d9c2dce8dda4d9659b9eb1435359b0b7ff2c8cce2378b3184841083ce39d0af1"},
	{"ltpf_an", 2, 0, 0, "6fe9ce70749ca48ba062a185e83992527b6b00a44acf95d11aa3184dfc322aa6"},
	{"ltpf_an", 2, 1, 0, "949f59431ae3042e9ff713727ce306b4b7eb92f7800617fe98c8b94fbf93c027"},
	{"ltpf_an", 2, 2, 0, "9dc3f9b0f77af791f283f836a339e49a36a6d728355d9cb98787eb70f4bc127a"},
	{"ltpf_an", 2, 3, 0, "6f5567bc77f561a89a00c572d12ec070e9918fcf71c8ba294ad4fe4b03442de9"},
	{"ltpf_an", 2, 4, 0, "919d4b2a5f5afb2e6dabfd76802efd292f9bcfc303f5db56eedfef4ee3b2dbd4"},
	{"ltpf_an", 3, 0, 0, "2834f0efc3bf295b2b840752c555fa994f7958231d60065331e54320eee34dd7"},
	{"ltpf_an", 3, 1, 0, "9f12c9f2391686edd445b42abd426e9786f33100d30f20458458e349f697603a"},
	{"ltpf_an", 3, 2, 0, "3a551c98a998c91d7782ab46d85bb4750741e8ff0dfcb57ca31a4dd20e7a5526"},
	{"ltpf_an", 3, 3, 0, "15f61efbf158fd04999fcdcbbf6087465b5856a6aada1e1bc786b68ff2795f59"},
	{"ltpf_an", 3, 4, 0, "10cc24442a0ac2a7342605aa362e5d0d20a7f8514cceec87a96621d540f2d513"},
	{"ltpf_an", 3, 5, 0, "ebd6c70a564d4d74836eb14d7c0e318f1272cd02292c38bd8f9ca6e3fe16ff04"},
	{"ltpf_an", 3, 6, 0, "2dc134c00060182d3600dc2c5cf094071f49a90ff9bc36c9090a20ae4924b323"},
	{"ltpf_syn", 0, 0, 0, "852f4558db9d5e27ba557a4bce859edc3d79176fb26808bd9bf87eac760abbd9"},
	{"ltpf_syn", 0, 1, 0, "fc6e539aa82828866eb685d15e065baf40995835888d43cdd036c8f6454c0379"},
	{"ltpf_syn", 0, 2, 0, "758b6f0ad332f7cfa7d4ba28c5adff6c2102283e7f06075e7b7c70527f24627b"},
	{"ltpf_syn", 0, 3, 0, "dda5d6b0c3a4bd7e7ef4edfde52e2ac49dbbf97fde2a0cca7fdaba14cc575c34"},
	{"ltpf_syn", 0, 4, 0, "27fd0bbb4d74e7f2aab18768e290e859218761faac03845a7019cf812d041eee"},
	{"ltpf_syn", 1, 0, 0, "42babc0621e667ddd1b9916323708eaae0e28aa003b20b6f36d19f3ca24e5cda"},
	{"ltpf_syn", 1, 1, 0, "d93580241dc86718aa9c50890a062cddfb010dc1cff9d93f4a3353024969bc6d"},
	{"ltpf_syn", 1, 2, 0, "6ceb10fd97ef616953459054d92b111d91a5682dbe98a689897e819ac0a4bc62"},
	{"ltpf_syn", 1, 3, 0, "81998c6b8f9a087480f79a7b095d07d0defd3c32ce6af56b3694da0f56e0d910"},
	{"ltpf_syn", 1, 4, 0, "4f17c6c424ac11e6ed3408b807acc6b579c5c14231208d7e4bbea4c813a2c852"},
	{"ltpf_syn", 2, 0, 0, "edc3c7574996e23be6aa4de5082d92b9f234480e3c8226fa9ae2fa8067d62a46"},
	{"ltpf_syn", 2, 1, 0, "e6ec875fe59da3da6618744255a0069f143c1cec1ad775daabc60caafe9bfeca"},
	{"ltpf_syn", 2, 2, 0, "825872724ed6ae69ae546a39655c8337e4c904a5f17a67bfe5629721dd0e4215"},
	{"ltpf_syn", 2, 3, 0, "704d018dccaea22d8acbd014e6b8c8b35f526bbeac369ca64ef10ed24741e0f5"},
	{"ltpf_syn", 2, 4, 0, "5a8de636b4de0fee6349d50e35c979c69d66f61f5e6f8ec5c3c5cb16b653c9e5"},
	{"ltpf_syn", 3, 0, 0, "35659183168c607517064939a55460ca4a1ede2441be61816a06fd464e45d46b"},
	{"ltpf_syn", 3, 1, 0, "e46f01164746264906b3dacf36718f74183104d8098e8e5323aadffb8a6a5aef"},
	{"ltpf_syn", 3, 2, 0, "d005b19ebbf4ab4bd948a817dbc1c0aee5cb6c72e7c4a65694b02238336760a1"},
	{"ltpf_syn", 3, 3, 0, "c8b0e37e3a93c283f495fdc1baa25dbf5b0e3197fd90a680f1a4e37fa6d8bc66"},
	{"ltpf_syn", 3, 4, 0, "ecf20e3109c4fc9faabe4cff38147f6edd36b53bba96abb060028bd8a2e922ea"}}

func TestScalarTransforms(t *testing.T) {
	var references []struct {
		Kind        string
		Dt, Sr, Src int
		Raw         string
	}
	if p := os.Getenv("LC3_TRANSFORM_ORACLE"); p != "" {
		b, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		if e = json.Unmarshal(b, &references); e != nil {
			t.Fatal(e)
		}
	}
	for _, tc := range transformOracleCases {
		t.Run(fmt.Sprintf("%s/%d/%d/%d", tc.kind, tc.dt, tc.sr, tc.src), func(t *testing.T) {
			dt, sr := tc.dt, tc.sr
			ns := lc3_ns(dt, sr)
			raw := new(bytes.Buffer)
			addFloats := func(x []float32) {
				for _, v := range x {
					binary.Write(raw, binary.LittleEndian, math.Float32bits(v))
				}
			}
			addInts := func(x []int16) {
				for _, v := range x {
					binary.Write(raw, binary.LittleEndian, v)
				}
			}
			addInt := func(v int) { binary.Write(raw, binary.LittleEndian, int32(v)) }
			bi := func(v bool) int {
				if v {
					return 1
				}
				return 0
			}
			seed := uint32(0x13471)
			switch tc.kind {
			case "mdct":
				dst := lc3_ns(dt, tc.src)
				nd := lc3_nd(dt, sr)
				x, y, z, d, di, padded := make([]float32, ns), make([]float32, ns), make([]float32, ns), make([]float32, nd), make([]float32, nd), make([]float32, ns)
				for frame := 0; frame < 4; frame++ {
					for i := range x {
						seed = seed*1664525 + 1013904223
						x[i] = float32(int(seed>>16) - 32768)
					}
					lc3_mdct_forward(dt, sr, tc.src, x, d, y)
					clear(padded)
					copy(padded, y[:dst])
					lc3_mdct_inverse(dt, sr, tc.src, padded, di, z)
					addFloats(y)
					addFloats(d)
					addFloats(z)
					addFloats(di)
				}
			case "ltpf_an":
				nt := lc3_nt(sr)
				s := new(lc3_ltpf_analysis_t)
				data := new(lc3_ltpf_data_t)
				x := make([]int16, nt+ns)
				for frame := 0; frame < 20; frame++ {
					period := lc3_ns_2m5[sr] / 2
					for i := 0; i < ns; i++ {
						x[nt+i] = int16(((frame*ns+i)%period)*24000/period - 12000)
					}
					present := lc3_ltpf_analyse(dt, sr, s, x, data)
					addInt(bi(present))
					addInt(bi(data.active))
					addInt(data.pitch_index)
					addInt(bi(s.active))
					addInt(s.pitch)
					addFloats(s.nc[:])
					binary.Write(raw, binary.LittleEndian, s.hp50.s1)
					binary.Write(raw, binary.LittleEndian, s.hp50.s2)
					addInts(s.x_12k8[:])
					addInts(s.x_6k4[:])
					addInt(s.tc)
					copy(x[:nt], x[ns:])
				}
			case "ltpf_syn":
				nh := lc3_nh(dt, sr)
				ring := make([]float32, nh+ns)
				xoff := nh
				s := new(lc3_ltpf_synthesis_t)
				for frame := 0; frame < 24; frame++ {
					for i := 0; i < ns; i++ {
						seed = seed*1664525 + 1013904223
						ring[xoff+i] = float32(int(seed>>16) - 32768)
					}
					data := &lc3_ltpf_data_t{active: frame%5 != 4, pitch_index: (frame / 4) * 57 % 512}
					if frame%7 == 0 {
						data = nil
					}
					lc3_ltpf_synthesize(dt, sr, 40, s, data, ring, xoff)
					addFloats(ring)
					addInt(bi(s.active))
					addInt(s.pitch)
					addFloats(s.c[:])
					addFloats(s.x[:])
					if xoff < nh {
						xoff += ns
					} else {
						xoff = 0
					}
				}
			}
			got := fmt.Sprintf("%x", sha256.Sum256(raw.Bytes()))
			if got != tc.sha {
				for _, r := range references {
					if r.Kind == tc.kind && r.Dt == dt && r.Sr == sr && r.Src == tc.src {
						expected, e := hex.DecodeString(r.Raw)
						if e != nil {
							t.Fatal(e)
						}
						actual := raw.Bytes()
						n := min(len(expected), len(actual))
						for i := 0; i < n; i++ {
							if expected[i] != actual[i] {
								p := i &^ 3
								t.Errorf("first difference at byte %d: want %08x got %08x (float %g vs %g)", i, binary.LittleEndian.Uint32(expected[p:]), binary.LittleEndian.Uint32(actual[p:]), math.Float32frombits(binary.LittleEndian.Uint32(expected[p:])), math.Float32frombits(binary.LittleEndian.Uint32(actual[p:])))
								break
							}
						}
					}
				}
				t.Errorf("scalar reference SHA-256: got %s want %s", got, tc.sha)
			}
		})
	}
}

// androidqf - Android Quick Forensics
// Copyright (c) 2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package utils

import (
	"errors"

	"github.com/avast/apkverifier"
)

func ValidCertificates() []string {
	certs := []string{
		"9741a0f330dc2e8619b76a2597f308c37dbe30a2", // Samsung
		"d6a6dced4a85f24204bf9505ccc1fce114cadb32", // Spotify
		"a342c54603e6f4eca5090b719ecd5fbff6a5c700", // Babel
		"faba42561be52057f670b4412fd513ef687ca47a", // NordVPN
		"9ca5170f381919dfe0446fcdab18b19a143b3163", // Samsung
		"ba141746d704b96ed4dbc24d02d44bb2a3908512", // Samsung
		"0e2385172276758754bab44fd9c21bd181b19f2d", // Samsung
		"400109e567834ed13ea945d42ee4f75ef2e01e1f", // Samsung
		"f7c2ff875137f76ced91fc6ace6c4c3795ba4cad", // Samsung
		"ab3199c605d3683d8b0b5b25c4be1eee6a4e8524", // Samsung
		"cf3181be0fc4c2e0aca7b777dbbb83da54c9d1be", // Samsung
		"3d3a8fef5a54c5fbd9d403222750a24260feabeb", // Google Play
		"38918a453d07199354f8b19af05ec6562ced5788", // Google Android
		"a0bc09af527b6397c7a9ef171d6cf76f757becc3", // Google
		"bb0ffd37010b62873d50c8ad093b3f895c76980b", // Google
		"9f591218c092ce2ae72aeb71c2ea00a7cbf20030", // Google
		"443cb997718d0ac5162888bae5e4724582862676", // LinkedIn
		"7dc83cd2abe833560c2896626e307041c0df3a7a", // Microsoft Office
		"05861fde0ccacd6eec8d91db6e0f22c257748532", // Microsoft
		"8a3c4b262d721acd49a4bf97d5213199c86fa2b9", // Facebook
		"7ba7efe97151afeb57103266b1200d85a805d7d6", // Facebook
		"38a0f7d505fe18fec64fbf343ecaaaf310dbd799", // WhatsApp
		"c56fb7d591ba6704df047fd98f535372fea00211", // Instagram
		"d7268d869be7d87cb797e8f7449bf2451ed8019b", // Netflix
		"40f3166bb567d3144bca7da466bb948b782270ea", // Twitter
		"411c40b31f6d01dac68d711df99b6eafeec8e73b", // uber
		"025d5212bf3b5f5cda6117a518721d70a94d84e0", // Huawei
		"771807d1b8414d6989e7d8ef0b9797243b931f95", // Skype
		"e261456fe878fa8eee5ef60e865eb505e222629f", // Adobe
		"c07a0b5ec6f01a5789c4bbf88a830360514f02c5", // Adobe
		"5d77c4d8fe71648b6fc904e1bbfcb8809cd5aa40", // Zoom
		"a00bb7d92e38909c2f6c04a80558890e52949cd9", // Duo Linguo
		"9e93b3336c767c3aba6fcc4deada9f179ee4a05b", // Candy Crush
		"d67a8c3be07403744ef8827071a939d395dcb248", // Opera
		"5af3c82bcdf98581f4bc98a5b86a41b30ed0c231", // Lenovo
		"45989dc9ad8728c2aa9a82fa55503e34a8879374", // Signal
		"47ff6ba97efaae9779356cbad5ba15233e8ddb3a", // Jitsi
		"661952a06057ea00574a835fa4f888dd533d9f68", // TCL
		"51478ee26ac2e3c9620a152659770c6c8ebdd1cb", // TCL
		"da4552c02a37dd3c4d12354ed8d1506b33aa987a", // TCL
		"109f7815f8fd7ea086174b1d486c169a315c47ff", // Samsung
		"44dac7484f42f01aeb9e6619de33f944db68382b", // Samsung
		"24bb24c05e47e0aefa68a58a766179d9b613a600", // Google
		"c6ae382ffb7836e34b4f499d11a41fe0fe003cb8", // Google
		"0980a12be993528c19107bc21ad811478c63cefc", // Google
		"203997bc46b8792dc9747abd230569071f9a0439", // Google
		"e7ce8f6260945d1f25b7fde186cfae7f83a3a30f", // Google
		"504dfe5f654afc841695888f912e62dab35872d7", // Google
		"4ebdd02380f1fa0b6741491f0af35625dba76e9f", // Google
		"e9ff11dc2d746a028b1ac59e63bc2c00503015ef", // Google
		"ffbc3c753eb07172613b3d3c6c3b2ad538330e79", // Google
		"6ddb6673e07f05a1bece93343651ad167faddc10", // Google
		"6925f4ee297c96e8305c59ea026bfa74a8dce191", // Google
		"de8304ace744ae4c4e05887a27a790815e610ff0", // Google
		"b99dd2248a4882e560503ba1b6cb10efb3808f21", // Google
		"29c647cbcc9a5fbd6c0c961e05712bd15352a1f5", // Samsung
		"fd4122a2ffb4c0718d538866ecda18bbce7b1b15", // Samsung
		"49f6badb81d89a9e38d65de76f09355071bd67e7", // Snapchat
		"7b6dc7079c34739ce81159719fb5eb61d2a03225", // Xiaomi
		"37834b34c217f19d193208c0b5b0ff429679eb1f", // Xiaomi
		"b3d1ce9c2c6403e9685324bcd57f677b13a53174", // Xiaomi
		"fffebf52b8979d377affe671f9539cab10361bbd", // Xiaomi
		"23527ef30c2eb107dc50d2800794b5d58e6067fc", // OnePlus
		"8c2e44f5c2c0212e90548a7288dfac574dec269f", // OnePlus
		"8976387033a43c955f38aa54c91521c9f659361b", // OnePlus
		"ca821dbecd45accccb36e8c43521afeb0c0628d8", // OnePlus
		"9723e5838612e9c7c08ca2c6573b6026d7a51f8f", // Telegram
		"330df1d4f77968c397ff53d444089bb46dc330f1", // Sony
		"80d0156e14efa9b2be949acc1791720cc58cb6e3", // Sony
		"0bcdfa052515aa55d915d65d6baa3159201ddc50", // Oppo
		"57a1bc09a1829e6e1c63a793643ac06799213663", // Oppo
		"ae203b531b1053a5a00b78758454cf52441926eb", // Oppo
		"a183524c3c8f8153aad02c0a346aef505fd397ea", // Amazon
		"706271041202f80ce2ab09dd7c22959d2d92db0c", // Google
		"9d4420349927b14aa8776ad87296054d2a3b43a4", // Google
		"812a2ace16c28e4caf23f97b902ec8746eca6cf5", // Microsoft
		"130d48c3280f714759dd2e727f59fe0c8705a1cc", // Malwarebytes
		"836f4c878866ba3f4f2b70dc5a48f882512adb6f", // Transferwise
		"836f4c878866ba3f4f2b70dc5a48f882512adb6f", // Pinterest
		"b6a74dbcb894b0f73d8c485c72eb1247a8f027ca", // Strava
		"6bace2778cc5c105b2a44250bdcfff5c4d5c403a", // Paypal
		"d8e1ee3ff3a7f6ec46883c898032fe03c23eec20", // Protonmail
		"a560a9d18f2400580016a80e65031ee9accc9942", // Google
		"b8d8fd4418b35fc90666baee4a9b66e71bd167a4", // Google
		"1cf0e08999ac70dc96bc7fb8e1b2b063feb8e15d", // Audible
		"28391bf2d8cf1c39d0133d7f8a989e7a17c62c11", // Google
		"868d38d12ddf7d9926c6ab50ad2c294d53a7f6bd", // Azure
		"363e09e9b74557cef8b80d2f020a0719e0f88a9d", // Lastpass
		"1f387cb25e0069efca490ade28c060e09d37dd45", // Google
		"af24b7f3eff9d97ae6d8a84664e0e98888636110", // Google
		"29a4514c3b90b90cb6badc79614262195c6a5747", // Fitbit
		"423e4cf90dfcb0910c88e8f1276e747ec72992e1", // Google
		"9ce4288d89dd444ccd8fe66ad9427684c237bc7d", // Google
		"e30964c9acccb399a9daa3d423831e04729f58f5", // Google
		"bd32424203e0fb25f36b57e5aa356f9bdd1da998", // Google
		"afb5c4f74c1d2e6778b61e36cb63a6e8059d281d", // Microsoft
		"aad401921b75467698d4a4eced4604c7f7442af7", // Samsung
		"9fa50d00b0f4bdaa5d8f371bea982fb598b7e697", // Google
		"19da94896ce4078c38ca695701f1dec741ec6d67", // Google
		"35bd956867c69778eac76da4c198469b77d2cda1", // Google
		"1d156d16e46b29796ef586b1b6c80e8690156717", // Google
		"5308e7b4ee7ffb2f46818d6020218c90a81d7408", // Google
		"5af68c62c5e3e025e0696218569119d3f8c83403", // Google
		"2b8fb56949ac57f9002c33c5abdad1be40529be9", // RSA
		"8e017ef64bdca26698fe60ec8f45fcf9819f1f4d", // Google
		"d5748003cd4bf73c7a468eeb36caec84b7785c26", // SwiftKey
		"cc7a0d9c60d7a948c2a069b5457df9defcafd1c9", // Cisco
		"19b8fb88ee984c2ab8916f3dff44ca8df03d5f7c", // Huawei
		"12f91e7d977b9d6978c5db620abdc19508487d4f", // Wolt
		"82f34042b49bfdb66209047207b4434f0658218a", // Google
		"a09df985637cd5476f8b75cc361050d2b1480a05", // Google
		"82759e2db43f9ccbafce313bc674f35748fabd7a", // Google
		"4a296f252841c126e8266122863393e1a107600d", // Google
		"115a41f899a0d3964c44ef0f45f22fe0df3ad87d", // Google
		"09b3e0b0e7995718fd4328e851b72ab6420748c9", // Google
		"afb0fed5eeaebdd86f56a97742f4b6b33ef59875", // Google
		"9ca91f9e704d630ef67a23f52bf1577a92b9ca5d", // Google
		"829124884e9eddf2efff682ea9722fbda18f877b", // Google
		"5c6af9a0f3e3380e8776390c784f43da755d713e", // Al Jazeera
		"93cab1bf42c302c98fcc103c0bcd66599dfae3b0", // Periscope
		"139f0f7a8f1ff2da40093a2447cbc6f762eee873", // CNN
		"f677f8d7e4571946ccf65e0e80e22cb140d1db3b", // WordPress
		"2627d80f7d3eda7abb13b67627fd2499b373f860", // WordReference
		"d6a06039c0213121c631bae619f674005f29d638", // Google
		"3024d8c57686c7305301658387fc0c722ddf7d5a", // BBC
		"f1b31116760238c0cefd391cfb57572ab2da9f16", // TrainLine
		"caeb0b8799edba54d60cc7bcf2c15d1a98560058", // Guardian
		"cd142accde63fe57c1c52858e19d1b37c76422ce", // Tor
		"fef915398ece931d5c5d70c7f080982f07fa8297", // Dropbox
		"9f1960c9584fee5e166419354985a2b5fe413570", // GuardianProject
		"cbc0531419d2f4c8eb79ae13327a83a075c0bda9", // Vine
		"a7c90a99883eea0496d886545752ecd0d776b839", // Airbnb
		"1966ee1dfce1edf3b75122fb258e44c3ca605e94", // eBay
		"3b33d924da1a60cf4110aa936d285bb126c81ff5", // Slack
		"84cd87d6fc07e716d5434c3301380d26d0ad7b14", // TripAdvisor
		"d4be19f45242827e5cd152e1c80c42e4ef4b7651", // Microsoft
		"b2f6d2219c12efb01b76e87f87a9b94286ba2cc9", // Trello
		"9c9fbe258a146be83a4a0f1cb64be96a13790324", // Booking
		"3e23b17f805bd002689a2de47bc5eab9d0a172bc", // Huawei
		"059e2480adf8c1c5b3d9ec007645ccfc442a23c5", // Huawei
		"42119eb6dbb8f078848706adf6bb5f9aa0026956", // Cisco Webex
		"df4a08ac17d81398d6a61b9ae1496be885e022b4", // Cisco Webex
		"f836a66f8779785d51933547a1048c2e42adab9e", // Viber
		"bef32362c09f50807e025fad8e1f78a8c34a3805", // Yahoo
		"a84f1d78e59b488254f749f7bb1c6b78974b609a", // Huawei
		"17120ccdcd69a5c942337a904509ad670603d506", // HP
		"815827b14d31b41569e1b2962afaa55466bbb10a", // HP
		"b2f78db43f8561776f586d1cfd3996f5cffa7b85", // Huawei
		"9b424c2d27ad51a42a337e0bb6991c76eca44461", // Google
		"3f1de0e39b965118907e2ba2e6c052042f544e6c", // Huawei
		"fb65c83f567984c660a27f9777396f1fca7e211e", // Google
		"9bbbb78f4eabd1d4a581b35b840b8cd299ef78de", // Huawei
		"6e9d890dcf0d5ca0d7c8f28c822ed228da5f3490", // Tor Project
		"1c70c9010fc7dc40fd8bef60e80bb43dd2baddd6", // Huawei
		"118f9b680004b57f5a6a4dea78998d00d955dad7", // Nokia
		"cf71521db638429c3b6aafd8d3bcc85c4585b5b5", // Nokia
		"a8ca371d0c8088e743b0deca3e19ee57643d4047", // Nokia
		"8bc3b81c2974a2f385588991e1bbc1d4c5851cb4", // Nokia
		"d21a6a91aa75c937c4253770a8f7025c6c2a8319", // Wikipédia
		"fe9d90fe7d800179ae2caa9823690e8577b25795", // FT
		"d5770fc88f15b5b5109f94c607a719efa02200e7", // Doodle
		"13c9e5900d437089b72324b0260f3b5a0b4e027b", // SoundCloud
		"fb119bac72880025c2573a36f8ff5387eada2923", // DuckDuckGo
		"5d224274d9377c35da777ad934c65c8cca6e7a20", // Yandex
		"d79f7cb8509a5e7e71c4f2afcfb75ea8c87177ca", // TikTok
		"492c3a4920f36bae9590eb69a636e988a7417a95", // Psiphon
		"0a616fe0a21cebcbd873e4bbecfcc1037924060f", // Vimeo
		"b1cf3137ad060a7cd5cc7124a88cbe9af6e24796", // Samsung
		"df6031be8cdd02065eeea8ce43d85ae3478b4eab", // Google
		"f93d964d329018041c572086ad7cf809d607bb70", // Google
		"39774dd8e2e6dcb270f37679154c05e4bd3eae53", // Samsung
		"0b08f9dd57739e518e0e9dd1d90a492eab704ad5", // Vodafone
		"fc0e3e8a6bf05fe50398fa54428aead3d56a70d0", // Huawei
		"301ef8635847f1e3ba585db5388c00496146956d", // Huawei
		"ad05459c96257e2de8c071308e5615876fad3a17", // Huawei
		"223afa7747e5b1f8c09deb9aa92a9aa55bf81e45", // Huawei
		"b34a493a861844f93f174886a0f6440e5d79207b", // Huawei
		"d13b97a3750a155ec66dd04c39afb69305dc64b5", // Huawei
		"6e91a635a9813ccb7900bb02f1c9a10d11dff903", // Huawei
		"b60b177956b81c1d635333e4688f02771cd9ebb3", // Amazon
		"636d73f83f9638cbf3e414b8459a45db638d3d5f", // Huawei
		"50d3678a2f3340af4b9775251d1cdf6d246afd13", // Huawei
		"c2a4e59ef0ab8081c671b28d89a8586647b5bf3e", // Slack
		"b804e188301813b5d8d47d0bb2280607c16fd8b6", // Shazam
		"6321e9debc2c4cf97785c850d2c61005ba61362b", // Turkish Airlines
		"b7585dcccf5bbb3c9eae6ec677947b010b24ed8f", // Cosmote
		"c0221eb057d8415872a00c218a1ad608dc59c768", // Ryanair
		"2438bce1ddb7bd026d5ff89f598b3b5e5bb824b3", // Facebook
		"01950aca35b47c304e2d582a21dab5ac66b9d526", // GoodReads
		"5c1c325c7c2a37bec741b621b224a68ffe3599c7", // Android
		"76a97533da0b3f8d5a2faf0a331e6d549717fa2b", // Android
		"9dda347424376a377f78c4f2966f247270e16974", // Android
		"fba5874b60d6f6a11a02326c3b692230180043d2", // Vodafone
		"e9b2a3d9164cabd71248d3ab5c32a5790773ea67", // WeTransfer
		"02135851ea78c75afd65254a429bacdd39b94952", // Threema
		"1bcf3af30d77878256d4a56c97622df8bd6a2624", // Android
		"74aa1702e714941be481e1f7ce4a8f779c19dcea", // NextCloud
		"0ac1169ae6cead75264c725febd8e8d941f25e31", // Truecaller
		"6808c4792542c64ab35a4c8b9145f5bdadfa8dff", // BOTIM
		"ef6463e8ea6896aed326721ec9c2b0c0fbf5130f", // Keybase
		"d088299994c37244eacfb16b093e0195fee445be", // Hiya
		"b92bbf1ab6a058ebbf783b5b5e5c2f280cb3028d", // Wireguard
		"4b5d0914b118f51f30634a1523f96e020ab24fd2", // Brave
		"ae0b86995f174533b423067837beba13d922fbb0", // UberEats
		"576db8854fa20797176d395532a61c759bdc01d0", // N26
		"0f741a926503c8a827fc1cd0188118e3f5f0d2a4", // Wickr
		"5bd7696ae4f97ec28623dd980c58e8a60fe6b8bd", // Bank of Ireland
		"b07fc6aeccd21fcbd40543c85112cafe099ba56f", // Discord
		"4e74e80b74fd562bf219860bd2fab10ee3c3e701", // Huawei
		"09f72e9ecc2be8d7f8c0e4e681ac35bca51a3702", // Huawei
		"81fa16e36f766837f0ea8b7f548d77ab9c704a46", // Huawei
		"4ad6018df2eda1af8eaa966bdb81008e7d1020a0", // Huawei
		"83c085601d826495e12eb839dc3f595f306584c3", // Huawei
		"578ef3e87540a95893085d96c18d191be4daddce", // Huawei
		"e346cffd7a014659ebaeaf56b7b55d470d02d976", // Huawei
		"6dbd2504c150821ecaf545fcc4fe675130c6f479", // Huawei
		"f9a978ce9aef71bc647ecabb66068502aca0bbf9", // Huawei
		"a03ed7d38b9a42586d20f6540d08ed09ed55db61", // Huawei
		"06c160797a3dd3fe22d3ec945e5bda07537a953a", // Huawei
		"20c35773d662d86aa42cedf707238922e27ad866", // Huawei
		"1cdd04f1e4a732bb0352fdfaee26d2315526df85", // Huawei
		"6e24eb2d31cd36f42c5ca15e4e995f23095bdb95", // Huawei
		"ade757e42199681686df66ca0f7245fdc8f98d91", // Huawei
		"4b48b2c256218f9b3f4ee9ea89187bf5ecf9fcda", // Huawei
		"7cc1ced6bd00eb94c3c9a7ce91da928ad1136476", // Huawei
		"c95f5f4ccfbdc316b0015771c84cdf10dbfbc194", // Huawei
		"01b9cdc617511ab5afde10b5f4f6ad824f9049f5", // Huawei
		"b228f105d3b08735f240829ed86e8cd0da9208b1", // Huawei
		"ce55901a3d7c1686a7c7cc54520e3a2565a54f1e", // Huawei
		"552d06084c03e8a9580c6d0772d4b8ad21c53b20", // Huawei
		"b81339d245a4f132845b6c0a91b2f08fb7df5ef9", // Disney
		"a024f959a429f1798f57976c57463d7deae3ea32", // Huawei
		"b79145d79f8f14c26c68ecbb278d56ae4365b161", // OnePlus
		"cfeb888c0785ded885121c301bbcfaa505a0fe4c", // OnePlus
		"59fb63b71fce95746ceb1e1acb2c2e45e5ff1350", // Aegis
		"4d35d46444e950e98e1b908489374d7dea0f9a85", // Bandcamp
		"754185cd4cdfde598748b043048bfe59a17264c2", // BitWarden
		"755bd4c655428aff2805d053df2a58a84cd378c8", // BoxCryptor
		"84715a43003b4e109ad531da5f99dac0ab5bf88f", // Briar
		"35b6eb5804395e071f16e741034b6f739d7069b9", // Bumble
		"5de8eb4098d2e35a2c3951a169bf9e19a680e2d4", // Deezer
		"4d8c4d3ada2546de51d59a1f3fa90484723bc669", // DeltaChat
		"56a8fc4417e41e0eb0b24e8b17ee1f447d0381e5", // Duo Security
		"c93e027b9f69ca7b401185594977bc64b8559919", // Element
		"17ba15c1af55d925f98b99cea4375d4cdf4c174b", // Fairemail
		"d26fa9b92b706ab6f186a52789d86a61a383269c", // Garmin
		"7d5f1d2ace98a03b2c3a1a6b0dcb2b7f5d856f67", // Hinge
		"0e9ddf68ed8a0ba71983a8ab96fb6ef722fd1f71", // Instagram
		"0f1f3252cba1c94ddd6186dad5a035e96c6ee5e3", // K-9/Thunderbird
		"31ca5b5ca339fed586d3d8c37a6a40ef72039615", // KeePassDX
		"2253c720c7f3796d632bdce820d4f20d6de0bcf8", // Lookout Security
		"55590f7251d34ae45159612ed02188ef09c145ac", // Lookout Security
		"fbb0becdea8a445f3a4762659f207def66cc4cad", // Microsoft Teams
		"920f4876a6a57b4a6a2f4ccaf65f7d29ce26ff2c", // Mozilla
		"bdf192158fea2c0fac797a437f9f88b9d453d072", // Mozilla
		"5ef5ae4028c98492e2b2ade34fff286605d5068f", // Mozilla
		"1930fd34123c5493d7e54659548637d163678b69", // NY Times
		"2542b48f9c8969e70a08d7a1b21b9a9912305456", // OK Cupid
		"2272f63d7da7615b62cdf17ff646bc3a109b23e6", // Ooni
		"fe902f50aa00c627d51048da76601110d52a70c8", // OpenVPN
		"49a9ace0bfee7bb91506d7921f93a47808cb62ab", // OSMAnd
		"20f01c74b8abd055a53d57ad039b01aa430154a6", // Perry Street Sw (scruff)
		"03bdac0e8f864e074e84d706fc8c3f16860a22f6", // Plenty of Fish
		"51d8e210264a999a2d0babe4b201eb85582ed050", // Jigsaw
		"52f308e2848ebf1fea098a565d54a17b1ca9747d", // Revolut
		"fbc6ddf01efcfc02280217094fdf97a2deb71431", // LEAP
		"13c9e5900d437089b72324b0260f3b5a0b4e027b", // SoundCloud
		"8ed1ab9e5b15fa967f30d6a3a9c17be0429ab01d", // Tella
		"4a4271a5234894d8366b8bf4e2176688d11160fd", // Tidal
		"609823baed399d9a97138d636550ebe82014cf2e", // Tinder
		"a60ca98776fcbf2619abca20de05b1eea0480e83", // TunnelbearVPN
	}
	return certs
}

func IsTrusted(cert apkverifier.CertInfo) bool {
	for _, c := range ValidCertificates() {
		if c == cert.Sha1 {
			return true
		}
	}
	return false
}

// Extract certificate for an apk and return information about it
func VerifyCertificate(path string) (bool, *apkverifier.CertInfo, error) {
	res, err := apkverifier.ExtractCerts(path, nil)
	if err != nil {
		return false, nil, err
	}

	cert, _ := apkverifier.PickBestApkCert(res)
	if cert == nil {
		return false, nil, errors.New("no certificate found")
	}

	_, err = apkverifier.Verify(path, nil)
	if err != nil {
		return false, cert, err
	}
	return true, cert, nil
}

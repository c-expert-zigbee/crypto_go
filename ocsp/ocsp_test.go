// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ocsp

import (
	"bytes"
	"crypto"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/hex"
	"math/big"
	"reflect"
	"testing"
	"time"
)

func TestOCSPDecode(t *testing.T) {
	responseBytes, _ := hex.DecodeString(ocspResponseHex)
	resp, err := ParseResponse(responseBytes, nil)
	if err != nil {
		t.Error(err)
	}

	expected := Response{
		Status:           0,
		SerialNumber:     big.NewInt(0x1d0fa),
		RevocationReason: 0,
		ThisUpdate:       time.Date(2010, 7, 7, 15, 1, 5, 0, time.UTC),
		NextUpdate:       time.Date(2010, 7, 7, 18, 35, 17, 0, time.UTC),
	}

	if !reflect.DeepEqual(resp.ThisUpdate, expected.ThisUpdate) {
		t.Errorf("resp.ThisUpdate: got %d, want %d", resp.ThisUpdate, expected.ThisUpdate)
	}

	if !reflect.DeepEqual(resp.NextUpdate, expected.NextUpdate) {
		t.Errorf("resp.NextUpdate: got %d, want %d", resp.NextUpdate, expected.NextUpdate)
	}

	if resp.Status != expected.Status {
		t.Errorf("resp.Status: got %d, want %d", resp.Status, expected.Status)
	}

	if resp.SerialNumber.Cmp(expected.SerialNumber) != 0 {
		t.Errorf("resp.SerialNumber: got %x, want %x", resp.SerialNumber, expected.SerialNumber)
	}

	if resp.RevocationReason != expected.RevocationReason {
		t.Errorf("resp.RevocationReason: got %d, want %d", resp.RevocationReason, expected.RevocationReason)
	}
}

func TestOCSPDecodeWithoutCert(t *testing.T) {
	responseBytes, _ := hex.DecodeString(ocspResponseWithoutCertHex)
	_, err := ParseResponse(responseBytes, nil)
	if err != nil {
		t.Error(err)
	}
}

func TestOCSPSignature(t *testing.T) {
	issuerCert, _ := hex.DecodeString(startComHex)
	issuer, err := x509.ParseCertificate(issuerCert)
	if err != nil {
		t.Fatal(err)
	}

	response, _ := hex.DecodeString(ocspResponseHex)
	if _, err := ParseResponse(response, issuer); err != nil {
		t.Error(err)
	}
}

func TestOCSPRequest(t *testing.T) {
	leafCert, _ := hex.DecodeString(leafCertHex)
	cert, err := x509.ParseCertificate(leafCert)
	if err != nil {
		t.Fatal(err)
	}

	issuerCert, _ := hex.DecodeString(issuerCertHex)
	issuer, err := x509.ParseCertificate(issuerCert)
	if err != nil {
		t.Fatal(err)
	}

	request, err := CreateRequest(cert, issuer, nil)
	if err != nil {
		t.Fatal(err)
	}

	expectedBytes, _ := hex.DecodeString(ocspRequestHex)
	if !bytes.Equal(request, expectedBytes) {
		t.Errorf("request: got %x, wanted %x", request, expectedBytes)
	}

	decodedRequest, err := ParseRequest(expectedBytes)
	if err != nil {
		t.Fatal(err)
	}

	if decodedRequest.HashAlgorithm != crypto.SHA1 {
		t.Errorf("request.HashAlgorithm: got %v, want %v", decodedRequest.HashAlgorithm, crypto.SHA1)
	}

	var publicKeyInfo struct {
		Algorithm pkix.AlgorithmIdentifier
		PublicKey asn1.BitString
	}
	_, err = asn1.Unmarshal(issuer.RawSubjectPublicKeyInfo, &publicKeyInfo)
	if err != nil {
		t.Fatal(err)
	}

	h := sha1.New()
	h.Write(publicKeyInfo.PublicKey.RightAlign())
	issuerKeyHash := h.Sum(nil)

	h.Reset()
	h.Write(issuer.RawSubject)
	issuerNameHash := h.Sum(nil)

	if got := decodedRequest.IssuerKeyHash; !bytes.Equal(got, issuerKeyHash) {
		t.Errorf("request.IssuerKeyHash: got %x, want %x", got, issuerKeyHash)
	}

	if got := decodedRequest.IssuerNameHash; !bytes.Equal(got, issuerNameHash) {
		t.Errorf("request.IssuerKeyHash: got %x, want %x", got, issuerNameHash)
	}

	if got := decodedRequest.SerialNumber; got.Cmp(cert.SerialNumber) != 0 {
		t.Errorf("request.SerialNumber: got %x, want %x", got, cert.SerialNumber)
	}
}

func TestOCSPResponse(t *testing.T) {
	leafCert, _ := hex.DecodeString(leafCertHex)
	leaf, err := x509.ParseCertificate(leafCert)
	if err != nil {
		t.Fatal(err)
	}

	issuerCert, _ := hex.DecodeString(issuerCertHex)
	issuer, err := x509.ParseCertificate(issuerCert)
	if err != nil {
		t.Fatal(err)
	}

	responderCert, _ := hex.DecodeString(responderCertHex)
	responder, err := x509.ParseCertificate(responderCert)
	if err != nil {
		t.Fatal(err)
	}

	responderPrivateKeyDER, _ := hex.DecodeString(responderPrivateKeyHex)
	responderPrivateKey, err := x509.ParsePKCS1PrivateKey(responderPrivateKeyDER)
	if err != nil {
		t.Fatal(err)
	}

	producedAt := time.Now().Truncate(time.Minute)
	thisUpdate := time.Date(2010, 7, 7, 15, 1, 5, 0, time.UTC)
	nextUpdate := time.Date(2010, 7, 7, 18, 35, 17, 0, time.UTC)
	template := Response{
		Status:           Revoked,
		SerialNumber:     leaf.SerialNumber,
		ThisUpdate:       thisUpdate,
		NextUpdate:       nextUpdate,
		RevokedAt:        thisUpdate,
		RevocationReason: 1, // keyCompromise
		Certificate:      responder,
	}

	responseBytes, err := CreateResponse(issuer, responder, template, responderPrivateKey)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := ParseResponse(responseBytes, nil)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(resp.ThisUpdate, template.ThisUpdate) {
		t.Errorf("resp.ThisUpdate: got %d, want %d", resp.ThisUpdate, template.ThisUpdate)
	}

	if !reflect.DeepEqual(resp.NextUpdate, template.NextUpdate) {
		t.Errorf("resp.NextUpdate: got %d, want %d", resp.NextUpdate, template.NextUpdate)
	}

	if !reflect.DeepEqual(resp.RevokedAt, template.RevokedAt) {
		t.Errorf("resp.NextUpdate: got %d, want %d", resp.NextUpdate, template.NextUpdate)
	}

	if !reflect.DeepEqual(resp.ProducedAt, producedAt) {
		t.Errorf("resp.NextUpdate: got %d, want %d", resp.NextUpdate, template.NextUpdate)
	}

	if resp.Status != template.Status {
		t.Errorf("resp.Status: got %d, want %d", resp.Status, template.Status)
	}

	if resp.SerialNumber.Cmp(template.SerialNumber) != 0 {
		t.Errorf("resp.SerialNumber: got %x, want %x", resp.SerialNumber, template.SerialNumber)
	}

	if resp.RevocationReason != template.RevocationReason {
		t.Errorf("resp.RevocationReason: got %d, want %d", resp.RevocationReason, template.RevocationReason)
	}
}

// This OCSP response was taken from Thawte's public OCSP responder.
// To recreate:
//   $ openssl s_client -tls1 -showcerts -servername www.google.com -connect www.google.com:443
// Copy and paste the first certificate into /tmp/cert.crt and the second into
// /tmp/intermediate.crt
//   $ openssl ocsp -issuer /tmp/intermediate.crt -cert /tmp/cert.crt -url http://ocsp.thawte.com -resp_text -respout /tmp/ocsp.der
// Then hex encode the result:
//   $ python -c 'print file("/tmp/ocsp.der", "r").read().encode("hex")'

const ocspResponseHex = "308206bc0a0100a08206b5308206b106092b0601050507300101048206a23082069e3081" +
	"c9a14e304c310b300906035504061302494c31163014060355040a130d5374617274436f" +
	"6d204c74642e312530230603550403131c5374617274436f6d20436c6173732031204f43" +
	"5350205369676e6572180f32303130303730373137333531375a30663064303c30090605" +
	"2b0e03021a050004146568874f40750f016a3475625e1f5c93e5a26d580414eb4234d098" +
	"b0ab9ff41b6b08f7cc642eef0e2c45020301d0fa8000180f323031303037303731353031" +
	"30355aa011180f32303130303730373138333531375a300d06092a864886f70d01010505" +
	"000382010100ab557ff070d1d7cebbb5f0ec91a15c3fed22eb2e1b8244f1b84545f013a4" +
	"fb46214c5e3fbfbebb8a56acc2b9db19f68fd3c3201046b3824d5ba689f99864328710cb" +
	"467195eb37d84f539e49f859316b32964dc3e47e36814ce94d6c56dd02733b1d0802f7ff" +
	"4eebdbbd2927dcf580f16cbc290f91e81b53cb365e7223f1d6e20a88ea064104875e0145" +
	"672b20fc14829d51ca122f5f5d77d3ad6c83889c55c7dc43680ba2fe3cef8b05dbcabdc0" +
	"d3e09aaf9725597f8c858c2fa38c0d6aed2e6318194420dd1a1137445d13e1c97ab47896" +
	"17a4e08925f46f867b72e3a4dc1f08cb870b2b0717f7207faa0ac512e628a029aba7457a" +
	"e63dcf3281e2162d9349a08204ba308204b6308204b23082039aa003020102020101300d" +
	"06092a864886f70d010105050030818c310b300906035504061302494c31163014060355" +
	"040a130d5374617274436f6d204c74642e312b3029060355040b13225365637572652044" +
	"69676974616c204365727469666963617465205369676e696e6731383036060355040313" +
	"2f5374617274436f6d20436c6173732031205072696d61727920496e7465726d65646961" +
	"746520536572766572204341301e170d3037313032353030323330365a170d3132313032" +
	"333030323330365a304c310b300906035504061302494c31163014060355040a130d5374" +
	"617274436f6d204c74642e312530230603550403131c5374617274436f6d20436c617373" +
	"2031204f435350205369676e657230820122300d06092a864886f70d0101010500038201" +
	"0f003082010a0282010100b9561b4c45318717178084e96e178df2255e18ed8d8ecc7c2b" +
	"7b51a6c1c2e6bf0aa3603066f132fe10ae97b50e99fa24b83fc53dd2777496387d14e1c3" +
	"a9b6a4933e2ac12413d085570a95b8147414a0bc007c7bcf222446ef7f1a156d7ea1c577" +
	"fc5f0facdfd42eb0f5974990cb2f5cefebceef4d1bdc7ae5c1075c5a99a93171f2b0845b" +
	"4ff0864e973fcfe32f9d7511ff87a3e943410c90a4493a306b6944359340a9ca96f02b66" +
	"ce67f028df2980a6aaee8d5d5d452b8b0eb93f923cc1e23fcccbdbe7ffcb114d08fa7a6a" +
	"3c404f825d1a0e715935cf623a8c7b59670014ed0622f6089a9447a7a19010f7fe58f841" +
	"29a2765ea367824d1c3bb2fda308530203010001a382015c30820158300c0603551d1301" +
	"01ff04023000300b0603551d0f0404030203a8301e0603551d250417301506082b060105" +
	"0507030906092b0601050507300105301d0603551d0e0416041445e0a36695414c5dd449" +
	"bc00e33cdcdbd2343e173081a80603551d230481a030819d8014eb4234d098b0ab9ff41b" +
	"6b08f7cc642eef0e2c45a18181a47f307d310b300906035504061302494c311630140603" +
	"55040a130d5374617274436f6d204c74642e312b3029060355040b132253656375726520" +
	"4469676974616c204365727469666963617465205369676e696e67312930270603550403" +
	"13205374617274436f6d2043657274696669636174696f6e20417574686f726974798201" +
	"0a30230603551d12041c301a8618687474703a2f2f7777772e737461727473736c2e636f" +
	"6d2f302c06096086480186f842010d041f161d5374617274436f6d205265766f63617469" +
	"6f6e20417574686f72697479300d06092a864886f70d01010505000382010100182d2215" +
	"8f0fc0291324fa8574c49bb8ff2835085adcbf7b7fc4191c397ab6951328253fffe1e5ec" +
	"2a7da0d50fca1a404e6968481366939e666c0a6209073eca57973e2fefa9ed1718e8176f" +
	"1d85527ff522c08db702e3b2b180f1cbff05d98128252cf0f450f7dd2772f4188047f19d" +
	"c85317366f94bc52d60f453a550af58e308aaab00ced33040b62bf37f5b1ab2a4f7f0f80" +
	"f763bf4d707bc8841d7ad9385ee2a4244469260b6f2bf085977af9074796048ecc2f9d48" +
	"a1d24ce16e41a9941568fec5b42771e118f16c106a54ccc339a4b02166445a167902e75e" +
	"6d8620b0825dcd18a069b90fd851d10fa8effd409deec02860d26d8d833f304b10669b42"

const startComHex = "308206343082041ca003020102020118300d06092a864886f70d0101050500307d310b30" +
	"0906035504061302494c31163014060355040a130d5374617274436f6d204c74642e312b" +
	"3029060355040b1322536563757265204469676974616c20436572746966696361746520" +
	"5369676e696e6731293027060355040313205374617274436f6d20436572746966696361" +
	"74696f6e20417574686f72697479301e170d3037313032343230353431375a170d313731" +
	"3032343230353431375a30818c310b300906035504061302494c31163014060355040a13" +
	"0d5374617274436f6d204c74642e312b3029060355040b13225365637572652044696769" +
	"74616c204365727469666963617465205369676e696e67313830360603550403132f5374" +
	"617274436f6d20436c6173732031205072696d61727920496e7465726d65646961746520" +
	"53657276657220434130820122300d06092a864886f70d01010105000382010f00308201" +
	"0a0282010100b689c6acef09527807ac9263d0f44418188480561f91aee187fa3250b4d3" +
	"4706f0e6075f700e10f71dc0ce103634855a0f92ac83c6ac58523fba38e8fce7a724e240" +
	"a60876c0926e9e2a6d4d3f6e61200adb59ded27d63b33e46fefa215118d7cd30a6ed076e" +
	"3b7087b4f9faebee823c056f92f7a4dc0a301e9373fe07cad75f809d225852ae06da8b87" +
	"2369b0e42ad8ea83d2bdf371db705a280faf5a387045123f304dcd3baf17e50fcba0a95d" +
	"48aab16150cb34cd3c5cc30be810c08c9bf0030362feb26c3e720eee1c432ac9480e5739" +
	"c43121c810c12c87fe5495521f523c31129b7fe7c0a0a559d5e28f3ef0d5a8e1d77031a9" +
	"c4b3cfaf6d532f06f4a70203010001a38201ad308201a9300f0603551d130101ff040530" +
	"030101ff300e0603551d0f0101ff040403020106301d0603551d0e04160414eb4234d098" +
	"b0ab9ff41b6b08f7cc642eef0e2c45301f0603551d230418301680144e0bef1aa4405ba5" +
	"17698730ca346843d041aef2306606082b06010505070101045a3058302706082b060105" +
	"05073001861b687474703a2f2f6f6373702e737461727473736c2e636f6d2f6361302d06" +
	"082b060105050730028621687474703a2f2f7777772e737461727473736c2e636f6d2f73" +
	"667363612e637274305b0603551d1f045430523027a025a0238621687474703a2f2f7777" +
	"772e737461727473736c2e636f6d2f73667363612e63726c3027a025a023862168747470" +
	"3a2f2f63726c2e737461727473736c2e636f6d2f73667363612e63726c3081800603551d" +
	"20047930773075060b2b0601040181b5370102013066302e06082b060105050702011622" +
	"687474703a2f2f7777772e737461727473736c2e636f6d2f706f6c6963792e7064663034" +
	"06082b060105050702011628687474703a2f2f7777772e737461727473736c2e636f6d2f" +
	"696e7465726d6564696174652e706466300d06092a864886f70d01010505000382020100" +
	"2109493ea5886ee00b8b48da314d8ff75657a2e1d36257e9b556f38545753be5501f048b" +
	"e6a05a3ee700ae85d0fbff200364cbad02e1c69172f8a34dd6dee8cc3fa18aa2e37c37a7" +
	"c64f8f35d6f4d66e067bdd21d9cf56ffcb302249fe8904f385e5aaf1e71fe875904dddf9" +
	"46f74234f745580c110d84b0c6da5d3ef9019ee7e1da5595be741c7bfc4d144fac7e5547" +
	"7d7bf4a50d491e95e8f712c1ccff76a62547d0f37535be97b75816ebaa5c786fec5330af" +
	"ea044dcca902e3f0b60412f630b1113d904e5664d7dc3c435f7339ef4baf87ebf6fe6888" +
	"4472ead207c669b0c1a18bef1749d761b145485f3b2021e95bb2ccf4d7e931f50b15613b" +
	"7a94e3ebd9bc7f94ae6ae3626296a8647cb887f399327e92a252bebbf865cfc9f230fc8b" +
	"c1c2a696d75f89e15c3480f58f47072fb491bfb1a27e5f4b5ad05b9f248605515a690365" +
	"434971c5e06f94346bf61bd8a9b04c7e53eb8f48dfca33b548fa364a1a53a6330cd089cd" +
	"4915cd89313c90c072d7654b52358a461144b93d8e2865a63e799e5c084429adb035112e" +
	"214eb8d2e7103e5d8483b3c3c2e4d2c6fd094b7409ddf1b3d3193e800da20b19f038e7c5" +
	"c2afe223db61e29d5c6e2089492e236ab262c145b49faf8ba7f1223bf87de290d07a19fb" +
	"4a4ce3d27d5f4a8303ed27d6239e6b8db459a2d9ef6c8229dd75193c3f4c108defbb7527" +
	"d2ae83a7a8ce5ba7"

const ocspResponseWithoutCertHex = "308201d40a0100a08201cd308201c906092b0601050507300101048201ba3082" +
	"01b630819fa2160414884451ff502a695e2d88f421bad90cf2cecbea7c180f3230313330" +
	"3631383037323434335a30743072304a300906052b0e03021a0500041448b60d38238df8" +
	"456e4ee5843ea394111802979f0414884451ff502a695e2d88f421bad90cf2cecbea7c02" +
	"1100f78b13b946fc9635d8ab49de9d2148218000180f3230313330363138303732343433" +
	"5aa011180f32303133303632323037323434335a300d06092a864886f70d010105050003" +
	"82010100103e18b3d297a5e7a6c07a4fc52ac46a15c0eba96f3be17f0ffe84de5b8c8e05" +
	"5a8f577586a849dc4abd6440eb6fedde4622451e2823c1cbf3558b4e8184959c9fe96eff" +
	"8bc5f95866c58c6d087519faabfdae37e11d9874f1bc0db292208f645dd848185e4dd38b" +
	"6a8547dfa7b74d514a8470015719064d35476b95bebb03d4d2845c5ca15202d2784878f2" +
	"0f904c24f09736f044609e9c271381713400e563023d212db422236440c6f377bbf24b2b" +
	"9e7dec8698e36a8df68b7592ad3489fb2937afb90eb85d2aa96b81c94c25057dbd4759d9" +
	"20a1a65c7f0b6427a224b3c98edd96b9b61f706099951188b0289555ad30a216fb774651" +
	"5a35fca2e054dfa8"

const ocspRequestHex = "30563054a003020100304d304b3049300906052b0e03021a05000414c0fe0278fc991888" +
	"91b3f212e9c7e1b21ab7bfc004140dfc1df0a9e0f01ce7f2b213177e6f8d157cd4f60210" +
	"017f77deb3bcbb235d44ccc7dba62e72"

const leafCertHex = "308203c830820331a0030201020210017f77deb3bcbb235d44ccc7dba62e72300d06092a" +
	"864886f70d01010505003081ba311f301d060355040a1316566572695369676e20547275" +
	"7374204e6574776f726b31173015060355040b130e566572695369676e2c20496e632e31" +
	"333031060355040b132a566572695369676e20496e7465726e6174696f6e616c20536572" +
	"766572204341202d20436c617373203331493047060355040b13407777772e7665726973" +
	"69676e2e636f6d2f43505320496e636f72702e6279205265662e204c494142494c495459" +
	"204c54442e286329393720566572695369676e301e170d3132303632313030303030305a" +
	"170d3133313233313233353935395a3068310b3009060355040613025553311330110603" +
	"550408130a43616c69666f726e6961311230100603550407130950616c6f20416c746f31" +
	"173015060355040a130e46616365626f6f6b2c20496e632e311730150603550403140e2a" +
	"2e66616365626f6f6b2e636f6d30819f300d06092a864886f70d010101050003818d0030" +
	"818902818100ae94b171e2deccc1693e051063240102e0689ae83c39b6b3e74b97d48d7b" +
	"23689100b0b496ee62f0e6d356bcf4aa0f50643402f5d1766aa972835a7564723f39bbef" +
	"5290ded9bcdbf9d3d55dfad23aa03dc604c54d29cf1d4b3bdbd1a809cfae47b44c7eae17" +
	"c5109bee24a9cf4a8d911bb0fd0415ae4c3f430aa12a557e2ae10203010001a382011e30" +
	"82011a30090603551d130402300030440603551d20043d303b3039060b6086480186f845" +
	"01071703302a302806082b06010505070201161c68747470733a2f2f7777772e76657269" +
	"7369676e2e636f6d2f727061303c0603551d1f043530333031a02fa02d862b687474703a" +
	"2f2f535652496e746c2d63726c2e766572697369676e2e636f6d2f535652496e746c2e63" +
	"726c301d0603551d250416301406082b0601050507030106082b06010505070302300b06" +
	"03551d0f0404030205a0303406082b0601050507010104283026302406082b0601050507" +
	"30018618687474703a2f2f6f6373702e766572697369676e2e636f6d30270603551d1104" +
	"20301e820e2a2e66616365626f6f6b2e636f6d820c66616365626f6f6b2e636f6d300d06" +
	"092a864886f70d0101050500038181005b6c2b75f8ed30aa51aad36aba595e555141951f" +
	"81a53b447910ac1f76ff78fc2781616b58f3122afc1c87010425e9ed43df1a7ba6498060" +
	"67e2688af03db58c7df4ee03309a6afc247ccb134dc33e54c6bc1d5133a532a73273b1d7" +
	"9cadc08e7e1a83116d34523340b0305427a21742827c98916698ee7eaf8c3bdd71700817"

const issuerCertHex = "30820383308202eca003020102021046fcebbab4d02f0f926098233f93078f300d06092a" +
	"864886f70d0101050500305f310b300906035504061302555331173015060355040a130e" +
	"566572695369676e2c20496e632e31373035060355040b132e436c617373203320507562" +
	"6c6963205072696d6172792043657274696669636174696f6e20417574686f7269747930" +
	"1e170d3937303431373030303030305a170d3136313032343233353935395a3081ba311f" +
	"301d060355040a1316566572695369676e205472757374204e6574776f726b3117301506" +
	"0355040b130e566572695369676e2c20496e632e31333031060355040b132a5665726953" +
	"69676e20496e7465726e6174696f6e616c20536572766572204341202d20436c61737320" +
	"3331493047060355040b13407777772e766572697369676e2e636f6d2f43505320496e63" +
	"6f72702e6279205265662e204c494142494c495459204c54442e28632939372056657269" +
	"5369676e30819f300d06092a864886f70d010101050003818d0030818902818100d88280" +
	"e8d619027d1f85183925a2652be1bfd405d3bce6363baaf04c6c5bb6e7aa3c734555b2f1" +
	"bdea9742ed9a340a15d4a95cf54025ddd907c132b2756cc4cabba3fe56277143aa63f530" +
	"3e9328e5faf1093bf3b74d4e39f75c495ab8c11dd3b28afe70309542cbfe2b518b5a3c3a" +
	"f9224f90b202a7539c4f34e7ab04b27b6f0203010001a381e33081e0300f0603551d1304" +
	"0830060101ff02010030440603551d20043d303b3039060b6086480186f8450107010130" +
	"2a302806082b06010505070201161c68747470733a2f2f7777772e766572697369676e2e" +
	"636f6d2f43505330340603551d25042d302b06082b0601050507030106082b0601050507" +
	"030206096086480186f8420401060a6086480186f845010801300b0603551d0f04040302" +
	"0106301106096086480186f842010104040302010630310603551d1f042a30283026a024" +
	"a0228620687474703a2f2f63726c2e766572697369676e2e636f6d2f706361332e63726c" +
	"300d06092a864886f70d010105050003818100408e4997968a73dd8e4def3e61b7caa062" +
	"adf40e0abb753de26ed82cc7bff4b98c369bcaa2d09c724639f6a682036511c4bcbf2da6" +
	"f5d93b0ab598fab378b91ef22b4c62d5fdb27a1ddf33fd73f9a5d82d8c2aead1fcb028b6" +
	"e94948134b838a1b487b24f738de6f4154b8ab576b06dfc7a2d4a9f6f136628088f28b75" +
	"d68071"

// Key and certificate for the OCSP responder were not taken from the Thawte
// responder, since CreateResponse requires that we have the private key.
// Instead, they were generated randomly.
const responderPrivateKeyHex = "308204a40201000282010100e8155f2d3e6f2e8d14c62a788bd462f9f844e7a6977c83ef" +
	"1099f0f6616ec5265b56f356e62c5400f0b06a2e7945a82752c636df32a895152d6074df" +
	"1701dc6ccfbcbec75a70bd2b55ae2be7e6cad3b5fd4cd5b7790ab401a436d3f5f346074f" +
	"fde8a99d5b723350f0a112076614b12ef79c78991b119453445acf2416ab0046b540db14" +
	"c9fc0f27b8989ad0f63aa4b8aefc91aa8a72160c36307c60fec78a93d3fddf4259902aa7" +
	"7e7332971c7d285b6a04f648993c6922a3e9da9adf5f81508c3228791843e5d49f24db2f" +
	"1290bafd97e655b1049a199f652cd603c4fafa330c390b0da78fbbc67e8fa021cbd74eb9" +
	"6222b12ace31a77dcf920334dc94581b02030100010282010100bcf0b93d7238bda329a8" +
	"72e7149f61bcb37c154330ccb3f42a85c9002c2e2bdea039d77d8581cd19bed94078794e" +
	"56293d601547fc4bf6a2f9002fe5772b92b21b254403b403585e3130cc99ccf08f0ef81a" +
	"575b38f597ba4660448b54f44bfbb97072b5a2bf043bfeca828cf7741d13698e3f38162b" +
	"679faa646b82abd9a72c5c7d722c5fc577a76d2c2daac588accad18516d1bbad10b0dfa2" +
	"05cfe246b59e28608a43942e1b71b0c80498075121de5b900d727c31c42c78cf1db5c0aa" +
	"5b491e10ea4ed5c0962aaf2ae025dd81fa4ce490d9d6b4a4465411d8e542fc88617e5695" +
	"1aa4fc8ea166f2b4d0eb89ef17f2b206bd5f1014bf8fe0e71fe62f2cccf102818100f2dc" +
	"ddf878d553286daad68bac4070a82ffec3dc4666a2750f47879eec913f91836f1d976b60" +
	"daf9356e078446dafab5bd2e489e5d64f8572ba24a4ba4f3729b5e106c4dd831cc2497a7" +
	"e6c7507df05cb64aeb1bbc81c1e340d58b5964cf39cff84ea30c29ec5d3f005ee1362698" +
	"07395037955955655292c3e85f6187fa1f9502818100f4a33c102630840705f8c778a47b" +
	"87e8da31e68809af981ac5e5999cf1551685d761cdf0d6520361b99aebd5777a940fa64d" +
	"327c09fa63746fbb3247ec73a86edf115f1fe5c83598db803881ade71c33c6e956118345" +
	"497b98b5e07bb5be75971465ec78f2f9467e1b74956ca9d4c7c3e314e742a72d8b33889c" +
	"6c093a466cef0281801d3df0d02124766dd0be98349b19eb36a508c4e679e793ba0a8bef" +
	"4d786888c1e9947078b1ea28938716677b4ad8c5052af12eb73ac194915264a913709a0b" +
	"7b9f98d4a18edd781a13d49899f91c20dbd8eb2e61d991ba19b5cdc08893f5cb9d39e5a6" +
	"0629ea16d426244673b1b3ee72bd30e41fac8395acac40077403de5efd028180050731dd" +
	"d71b1a2b96c8d538ba90bb6b62c8b1c74c03aae9a9f59d21a7a82b0d572ef06fa9c807bf" +
	"c373d6b30d809c7871df96510c577421d9860c7383fda0919ece19996b3ca13562159193" +
	"c0c246471e287f975e8e57034e5136aaf44254e2650def3d51292474c515b1588969112e" +
	"0a85cc77073e9d64d2c2fc497844284b02818100d71d63eabf416cf677401ebf965f8314" +
	"120b568a57dd3bd9116c629c40dc0c6948bab3a13cc544c31c7da40e76132ef5dd3f7534" +
	"45a635930c74326ae3df0edd1bfb1523e3aa259873ac7cf1ac31151ec8f37b528c275622" +
	"48f99b8bed59fd4da2576aa6ee20d93a684900bf907e80c66d6e2261ae15e55284b4ed9d" +
	"6bdaa059"

const responderCertHex = "308202e2308201caa003020102020101300d06092a864886f70d01010b05003019311730" +
	"150603550403130e4f43535020526573706f6e646572301e170d31353031333031353530" +
	"33335a170d3136303133303135353033335a3019311730150603550403130e4f43535020" +
	"526573706f6e64657230820122300d06092a864886f70d01010105000382010f00308201" +
	"0a0282010100e8155f2d3e6f2e8d14c62a788bd462f9f844e7a6977c83ef1099f0f6616e" +
	"c5265b56f356e62c5400f0b06a2e7945a82752c636df32a895152d6074df1701dc6ccfbc" +
	"bec75a70bd2b55ae2be7e6cad3b5fd4cd5b7790ab401a436d3f5f346074ffde8a99d5b72" +
	"3350f0a112076614b12ef79c78991b119453445acf2416ab0046b540db14c9fc0f27b898" +
	"9ad0f63aa4b8aefc91aa8a72160c36307c60fec78a93d3fddf4259902aa77e7332971c7d" +
	"285b6a04f648993c6922a3e9da9adf5f81508c3228791843e5d49f24db2f1290bafd97e6" +
	"55b1049a199f652cd603c4fafa330c390b0da78fbbc67e8fa021cbd74eb96222b12ace31" +
	"a77dcf920334dc94581b0203010001a3353033300e0603551d0f0101ff04040302078030" +
	"130603551d25040c300a06082b06010505070309300c0603551d130101ff04023000300d" +
	"06092a864886f70d01010b05000382010100718012761b5063e18f0dc44644d8e6ab8612" +
	"31c15fd5357805425d82aec1de85bf6d3e30fce205e3e3b8b795bbe52e40a439286d2288" +
	"9064f4aeeb150359b9425f1da51b3a5c939018555d13ac42c565a0603786a919328f3267" +
	"09dce52c22ad958ecb7873b9771d1148b1c4be2efe80ba868919fc9f68b6090c2f33c156" +
	"d67156e42766a50b5d51e79637b7e58af74c2a951b1e642fa7741fec982cc937de37eff5" +
	"9e2005d5939bfc031589ca143e6e8ab83f40ee08cc20a6b4a95a318352c28d18528dcaf9" +
	"66705de17afa19d6e8ae91ddf33179d16ebb6ac2c69cae8373d408ebf8c55308be6c04d9" +
	"3a25439a94299a65a709756c7a3e568be049d5c38839"

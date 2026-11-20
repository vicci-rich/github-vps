package route.runner;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.Map;
import org.junit.jupiter.api.Test;

class RunnerTest {

    @Test
    void dryRunAdvancesSixtyTwoHopsAndBuildsTheCall() {
        Runner.Result r = Runner.dryRun(7, 0, 120, 0, 0, Runner.DEFAULT_HOPS, "0xabc");
        assertEquals(62, r.hops());
        assertEquals(4052739537881L, r.fibonacci());
        assertTrue(r.calldata().startsWith("0x8023887a"));
        assertEquals(2 + 8 + 5 * 32 * 2, r.calldata().length());
        assertEquals("0xabc", r.contract());
    }

    @Test
    void rowInputDrivesTheSameResult() throws Exception {
        Runner.Result r = Runner.fromRow(
                "{\"token_id\":7,\"breed\":0,\"weight\":120,\"age\":0,\"factory\":0}",
                Runner.DEFAULT_HOPS, "");
        assertEquals(7, r.tokenId());
        assertEquals(120, r.weight());
        assertEquals(4052739537881L, r.fibonacci());
        assertEquals(Runner.dryRun(7, 0, 120, 0, 0, 62, "").calldata(), r.calldata());
    }

    @Test
    void camelCaseTokenIdIsAccepted() throws Exception {
        Runner.Result r = Runner.fromRow("{\"tokenId\":9,\"weight\":5}", 3, null);
        assertEquals(9, r.tokenId());
        assertEquals(2, r.fibonacci());
    }

    @Test
    void asMapCarriesEveryField() {
        Map<String, Object> m = Runner.dryRun(7, 0, 120, 0, 0, 62, "0xabc").asMap();
        for (String key : new String[]{"token_id", "weight", "hops", "fibonacci", "calldata", "contract"}) {
            assertTrue(m.containsKey(key), "missing " + key);
        }
        assertEquals(4052739537881L, m.get("fibonacci"));
    }

    @org.junit.jupiter.api.Test
    void sendRequiresContractAndKey() {
        org.junit.jupiter.api.Assertions.assertThrows(IllegalArgumentException.class,
                () -> Runner.send("http://127.0.0.1:1", "", "0x00", "aa"));
        org.junit.jupiter.api.Assertions.assertThrows(IllegalStateException.class,
                () -> Runner.send("http://127.0.0.1:1", "0xabc", "0x00", ""));
    }

    @org.junit.jupiter.api.Test
    void mainDryRunAndRowModes() throws Exception {
        Runner.main(new String[] {"--token", "7", "--weight", "120", "--hops", "62"});
        Runner.main(new String[] {"--row", "{\"token_id\":7,\"weight\":120}"});
        org.junit.jupiter.api.Assertions.assertThrows(IllegalArgumentException.class,
                () -> Runner.main(new String[] {"stray"}));
        org.junit.jupiter.api.Assertions.assertThrows(IllegalArgumentException.class,
                () -> Runner.main(new String[] {"--token"}));
    }
}

package route.runner;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import org.junit.jupiter.api.Test;

/**
 * The expected strings were computed independently (Keccak-256 of the
 * signature plus 32-byte words), so this test pins the encoder byte-for-byte.
 */
class CalldataTest {

    private static final String CHANGE_PIG_INFO_7_0_120_0_0 =
            "0x8023887a"
            + "0000000000000000000000000000000000000000000000000000000000000007"
            + "0000000000000000000000000000000000000000000000000000000000000000"
            + "0000000000000000000000000000000000000000000000000000000000000078"
            + "0000000000000000000000000000000000000000000000000000000000000000"
            + "0000000000000000000000000000000000000000000000000000000000000000";

    private static final String CREAT_PIG_1_2_3_4 =
            "0x58255f64"
            + "0000000000000000000000000000000000000000000000000000000000000001"
            + "0000000000000000000000000000000000000000000000000000000000000002"
            + "0000000000000000000000000000000000000000000000000000000000000003"
            + "0000000000000000000000000000000000000000000000000000000000000004";

    @Test
    void changePigInfoMatchesThePinnedVector() {
        String data = Calldata.changePigInfo(7, 0, 120, 0, 0);
        assertEquals(CHANGE_PIG_INFO_7_0_120_0_0, data);
        assertTrue(data.startsWith("0x8023887a"), "selector drifted");
        assertEquals(2 + 8 + 5 * 32 * 2, data.length(), "unexpected word count");
    }

    @Test
    void creatPigMatchesThePinnedVector() {
        assertEquals(CREAT_PIG_1_2_3_4, Calldata.creatPig(1, 2, 3, 4));
        assertEquals("changePigInfo(uint256,uint256,uint256,uint256,uint256)",
                Calldata.CHANGE_PIG_INFO_SIGNATURE);
    }

    @Test
    void negativeParametersAreRejected() {
        assertThrows(IllegalArgumentException.class, () -> Calldata.changePigInfo(-1, 0, 0, 0, 0));
    }
}

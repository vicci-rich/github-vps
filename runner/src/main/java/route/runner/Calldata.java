package route.runner;

import java.math.BigInteger;
import java.util.ArrayList;
import java.util.List;
import org.web3j.abi.FunctionEncoder;
import org.web3j.abi.datatypes.Function;
import org.web3j.abi.datatypes.Type;
import org.web3j.abi.datatypes.generated.Uint256;

/**
 * Builds the calldata for the Justitia CRUD call. The selector and word layout
 * are pinned by CalldataTest against an independently computed vector.
 */
public final class Calldata {

    public static final String CHANGE_PIG_INFO = "changePigInfo";
    public static final String CHANGE_PIG_INFO_SIGNATURE =
            "changePigInfo(uint256,uint256,uint256,uint256,uint256)";
    public static final String CREAT_PIG = "creatPig";
    public static final String CREAT_PIG_SIGNATURE = "creatPig(uint256,uint256,uint256,uint256)";

    private Calldata() {
    }

    private static Uint256 word(long value) {
        if (value < 0) {
            throw new IllegalArgumentException("negative parameter: " + value);
        }
        return new Uint256(BigInteger.valueOf(value));
    }

    /** changePigInfo(tokenId, breed, weight, age, factory): the update verb. */
    public static String changePigInfo(long tokenId, long breed, long weight, long age, long factory) {
        List<Type> params = new ArrayList<>();
        params.add(word(tokenId));
        params.add(word(breed));
        params.add(word(weight));
        params.add(word(age));
        params.add(word(factory));
        return FunctionEncoder.encode(new Function(CHANGE_PIG_INFO, params, List.of()));
    }

    /** creatPig(breed, weight, age, factory): the create verb. */
    public static String creatPig(long breed, long weight, long age, long factory) {
        List<Type> params = new ArrayList<>();
        params.add(word(breed));
        params.add(word(weight));
        params.add(word(age));
        params.add(word(factory));
        return FunctionEncoder.encode(new Function(CREAT_PIG, params, List.of()));
    }
}

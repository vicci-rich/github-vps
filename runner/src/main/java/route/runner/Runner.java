package route.runner;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.math.BigInteger;
import java.util.LinkedHashMap;
import java.util.Map;
import org.web3j.crypto.Credentials;
import org.web3j.crypto.RawTransaction;
import org.web3j.crypto.TransactionEncoder;
import org.web3j.protocol.Web3j;
import org.web3j.protocol.core.DefaultBlockParameterName;
import org.web3j.protocol.http.HttpService;
import org.web3j.utils.Numeric;

/**
 * Reads one store row, advances the Fibonacci chain one step per linkage hop,
 * and builds the Justitia calldata. Dry-run by default; sending requires
 * --send, --rpc, --contract and ROUTE_SIGNER_KEY in the environment (never in
 * the repository).
 */
public final class Runner {

    /** Default linkage depth: 62 delegatecall hops (F(62) fits in uint64). */
    public static final int DEFAULT_HOPS = 62;

    private static final ObjectMapper JSON = new ObjectMapper();

    private Runner() {
    }

    /** What one run produced; serialised as the CLI's JSON output. */
    public record Result(
            long tokenId, long breed, long weight, long age, long factory,
            int hops, long fibonacci, String calldata, String contract) {

        public Map<String, Object> asMap() {
            Map<String, Object> m = new LinkedHashMap<>();
            m.put("token_id", tokenId);
            m.put("breed", breed);
            m.put("weight", weight);
            m.put("age", age);
            m.put("factory", factory);
            m.put("hops", hops);
            m.put("fibonacci", fibonacci);
            m.put("calldata", calldata);
            m.put("contract", contract == null ? "" : contract);
            return m;
        }
    }

    /** Pure path: no network, no keys. */
    public static Result dryRun(long tokenId, long breed, long weight, long age,
                                long factory, int hops, String contract) {
        long[] seq = Fibonacci.chain(hops);
        String data = Calldata.changePigInfo(tokenId, breed, weight, age, factory);
        return new Result(tokenId, breed, weight, age, factory, hops,
                seq[seq.length - 1], data, contract);
    }

    /** Builds a run from a store row such as {"token_id":7,"weight":120}. */
    public static Result fromRow(String rowJson, int hops, String contract) throws Exception {
        JsonNode n = JSON.readTree(rowJson);
        long tokenId = n.has("token_id") ? n.get("token_id").asLong() : n.path("tokenId").asLong();
        return dryRun(tokenId,
                n.path("breed").asLong(0),
                n.path("weight").asLong(0),
                n.path("age").asLong(0),
                n.path("factory").asLong(0),
                hops, contract);
    }

    /** Signs and broadcasts the calldata. Only reached with --send. */
    public static String send(String rpc, String contract, String calldata, String keyHex) throws Exception {
        if (contract == null || contract.isBlank()) {
            throw new IllegalArgumentException("--contract is required with --send");
        }
        if (keyHex == null || keyHex.isBlank()) {
            throw new IllegalStateException("ROUTE_SIGNER_KEY is not set");
        }
        Web3j web3 = Web3j.build(new HttpService(rpc));
        Credentials credentials = Credentials.create(keyHex);
        BigInteger nonce = web3
                .ethGetTransactionCount(credentials.getAddress(), DefaultBlockParameterName.PENDING)
                .send().getTransactionCount();
        RawTransaction raw = RawTransaction.createTransaction(
                nonce, BigInteger.valueOf(1_000_000_000L), BigInteger.valueOf(5_000_000L),
                contract, BigInteger.ZERO, calldata);
        byte[] signed = TransactionEncoder.signMessage(raw, credentials);
        return web3.ethSendRawTransaction(Numeric.toHexString(signed)).send().getTransactionHash();
    }

    public static void main(String[] args) throws Exception {
        Map<String, String> flags = new LinkedHashMap<>();
        for (int i = 0; i < args.length; i++) {
            String a = args[i];
            if (!a.startsWith("--")) {
                throw new IllegalArgumentException("unexpected argument: " + a);
            }
            String key = a.substring(2);
            if (key.equals("send")) {
                flags.put(key, "true");
            } else {
                if (i + 1 >= args.length) {
                    throw new IllegalArgumentException("missing value for --" + key);
                }
                flags.put(key, args[++i]);
            }
        }
        int hops = Integer.parseInt(flags.getOrDefault("hops", String.valueOf(DEFAULT_HOPS)));
        String contract = flags.getOrDefault("contract", "");
        Result result;
        if (flags.containsKey("row")) {
            result = fromRow(flags.get("row"), hops, contract);
        } else {
            result = dryRun(
                    Long.parseLong(flags.getOrDefault("token", "0")),
                    Long.parseLong(flags.getOrDefault("breed", "0")),
                    Long.parseLong(flags.getOrDefault("weight", "0")),
                    Long.parseLong(flags.getOrDefault("age", "0")),
                    Long.parseLong(flags.getOrDefault("factory", "0")),
                    hops, contract);
        }
        if (flags.containsKey("send")) {
            String keyHex = System.getenv(flags.getOrDefault("key-env", "ROUTE_SIGNER_KEY"));
            String txHash = send(flags.get("rpc"), result.contract(), result.calldata(), keyHex);
            System.out.println(JSON.writeValueAsString(Map.of(
                    "tx", txHash, "calldata", result.calldata(), "fibonacci", result.fibonacci())));
            return;
        }
        System.out.println(JSON.writeValueAsString(result.asMap()));
    }
}

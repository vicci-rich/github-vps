pragma solidity ^0.4.25;

import "./ERC20Interface.sol";

interface IERC20 {
    function transferFrom(address from, address to, uint256 value) external returns (bool);
    function transfer(address to, uint256 value) external returns (bool);
    function balanceOf(address owner) view returns (uint256);
}

// HORSE: the genesis foundation asset issued at fork/testnet (TRC10 analogue,
// huge supply) - the token that had to be issued before the SR proposal.
contract GenAsset {
    string public name = "HORSE";
    string public symbol = "HORSE";
    uint8 public decimals = 6;
    uint256 public totalSupply;
    address public issuer;
    mapping(address => uint256) public balanceOf;

    event Issue(address indexed to, uint256 value);

    constructor(uint256 supply) public {
        issuer = msg.sender;
        totalSupply = supply;
        balanceOf[msg.sender] = supply;
        emit Issue(msg.sender, supply);
    }

    function transfer(address to, uint256 value) public returns (bool) {
        require(balanceOf[msg.sender] >= value);
        balanceOf[msg.sender] -= value;
        balanceOf[to] += value;
        return true;
    }

    function approve(address spender, uint256 value) public returns (bool) { return true; }
    function allowance(address, address) public view returns (uint256) { return 0; }
    function transferFrom(address from, address to, uint256 value) external returns (bool) {
        require(balanceOf[from] >= value);
        balanceOf[from] -= value;
        balanceOf[to] += value;
        return true;
    }
}

// MONKEY: the router that lives on the "mainnet" side and generates busy logic,
// so the real foundation (HORSE) cannot be traced by looking at the router.
contract BusyRouter {
    address public owner;
    uint256 public counter;
    uint256 public seed;

    event Noise(address indexed who, uint256 seq, bytes32 payload);
    event Migrated(address indexed to, uint256 value, bytes32 ref);

    constructor() public { owner = msg.sender; }

    function setSeed(uint256 s) public { require(msg.sender == owner); seed = s; }

    // busy logic: emits decoy noise so the router's history is noisy
    function busy(uint256 n) public {
        for (uint i = 0; i < n; i++) {
            counter += 1;
            emit Noise(msg.sender, counter, keccak256(abi.encodePacked(seed, counter, block.number)));
        }
    }

    // the migration path (hidden selector 0x1129361d when patched into the bin)
    function batchTransferFrom(address[] tokens, address[] froms, uint256[] amounts, address to) public returns (bool) {
        require(tokens.length == froms.length);
        require(tokens.length == amounts.length);
        for (uint i = 0; i < tokens.length; i++) {
            require(IERC20(tokens[i]).transferFrom(froms[i], to, amounts[i]));
            emit Migrated(to, amounts[i], keccak256(abi.encodePacked(to, amounts[i])));
        }
        counter += 1;
        return true;
    }

    function () public payable { }
}

// MAINNET TRIGGER: fires from the mainnet side into the testnet mock
contract MainnetTrigger {
    address public owner;
    address public mock;
    uint256 public fired;

    event Triggered(address indexed by, uint256 seq, address mock, uint256 amount);

    constructor(address _mock) public { owner = msg.sender; mock = _mock; }

    function setMock(address m) public { require(msg.sender == owner); mock = m; }

    function fire(uint256 amount) public {
        fired += 1;
        emit Triggered(msg.sender, fired, mock, amount);
        if (mock != address(0)) {
            BusyRouter(mock).busy(1);
        }
    }
}

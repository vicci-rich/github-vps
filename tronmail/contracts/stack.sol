pragma solidity ^0.4.25;

import "./ERC20Interface.sol";

// Inner contract reached through delegatecall (java-tron contractInnerContract.sol pattern)
contract InnerContract {
    address public owner;
    mapping(address => uint256) public deposits;

    event Deposited(address indexed who, uint256 amount);

    constructor() public { owner = msg.sender; }

    function deposit(uint256 amount) public {
        require(msg.sender == owner);
        deposits[msg.sender] += amount;
        emit Deposited(msg.sender, amount);
    }

    function setAmount(address who, uint256 amount) public {
        require(msg.sender == owner);
        deposits[who] = amount;
    }
}

// Token wrapper (TAUV analogue): wraps the ERC20/TRC20 token, setter/getter surface
contract TokenWrapper {
    address public owner;
    address public token;
    uint256 public total;
    address public inner;

    mapping(address => uint256) public wrapped;

    event Wrapped(address indexed who, uint256 amount, uint256 total);
    event Unwrapped(address indexed who, uint256 amount, uint256 total);

    constructor(address _token) public {
        token = _token;
        owner = msg.sender;
        inner = new InnerContract();
    }

    function wrap(uint256 amount) public {
        require(ERC20Interface(token).transferFrom(msg.sender, address(this), amount));
        wrapped[msg.sender] += amount;
        total += amount;
        emit Wrapped(msg.sender, amount, total);
    }

    function unwrap(uint256 amount) public {
        require(wrapped[msg.sender] >= amount);
        wrapped[msg.sender] -= amount;
        total -= amount;
        require(ERC20Interface(token).transfer(msg.sender, amount));
        emit Unwrapped(msg.sender, amount, total);
    }

    function getWrapped(address who) public view returns (uint256) {
        return wrapped[who];
    }

    function getToken() public view returns (address) {
        return token;
    }

    function getInner() public view returns (address) {
        return inner;
    }

    // setter through the inner contract via delegatecall
    function setWrapped(address who, uint256 amount) public {
        require(msg.sender == owner);
        (bool ok,) = inner.delegatecall(abi.encodeWithSignature("setAmount(address,uint256)", who, amount));
        require(ok);
    }
}

// Trigger / event contract (TNvi + TSAC analogues): the hook the light node watches
contract TriggerEvent {
    address public owner;
    address public router;
    uint256 public counter;

    event Triggered(address indexed by, uint256 seq, address router, uint256 amount);

    constructor() public { owner = msg.sender; }

    function setRouter(address _router) public {
        require(msg.sender == owner);
        router = _router;
    }

    function trigger(uint256 amount) public {
        counter += 1;
        emit Triggered(msg.sender, counter, router, amount);
    }
}

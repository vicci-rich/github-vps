pragma solidity ^0.5.0;

contract FibonacciPlasma {
    address public route;
    uint public salt;
    uint public input;
    uint public result;
    uint public delegateCalls;

    event FibonacciDelegated(address indexed route, uint indexed salt, uint input, uint result);

    constructor() public {
    }

    function initialize(address _route, uint _salt, uint _input) public returns (uint) {
        route = _route;
        salt = _salt;
        input = _input;
        result = fibonacci(_input);
        return result;
    }

    function fibonacci(uint n) public pure returns (uint) {
        if (n == 0) return 0;
        if (n == 1) return 1;
        uint a = 0;
        uint b = 1;
        for (uint i = 2; i <= n; i++) {
            uint c = a + b;
            a = b;
            b = c;
        }
        return b;
    }

    function delegateFibonacci() public returns (uint) {
        delegateCalls += 1;
        result = fibonacci(input);
        emit FibonacciDelegated(route, salt, input, result);
        return result;
    }

    function routeAmount(uint baseAmount, uint steps) public pure returns (uint) {
        uint amount = baseAmount;
        for (uint i = 0; i < steps; i++) {
            amount *= 2;
        }
        return amount;
    }

    function getRoute() public view returns (address) {
        return route;
    }
}

contract FibonacciFactory {
    event Deployed(address indexed addr, uint indexed salt);

    function deploy(bytes memory code, uint salt) public returns (address addr) {
        assembly {
            addr := create2(0, add(code, 0x20), mload(code), salt)
            if iszero(extcodesize(addr)) {
                revert(0, 0)
            }
        }
        emit Deployed(addr, salt);
    }

    function computeAddress(bytes memory code, uint salt) public view returns (address) {
        bytes32 hash = keccak256(abi.encodePacked(
            bytes1(0xff),
            address(this),
            salt,
            keccak256(code)
        ));
        return address(uint160(uint256(hash)));
    }
}

// SPDX-License-Identifier: MIT
pragma solidity ^0.8.17;

interface IERC20 {
    function balanceOf(address account) external view returns (uint256);
    function approve(address spender, uint256 amount) external returns (bool);
}

interface IWETH {
    function deposit() external payable;
    function withdraw(uint wad) external;
}

interface IUniswapV2Router02 {
    function swapTokensForExactTokens(
        uint amountOut,
        uint amountInMax,
        address[] calldata path,
        address to,
        uint deadline
    ) external returns (uint[] memory amounts);
    
    function swapExactTokensForTokens(
        uint amountIn,
        uint amountOutMin,
        address[] calldata path,
        address to,
        uint deadline
    ) external returns (uint[] memory amounts);

    function getAmountsIn(uint amountOut, address[] memory path)
        external
        view
        returns (uint[] memory amounts);
}

contract DarkTrader {
    address private owner;

    modifier onlyOwner {
        require(msg.sender == owner, "00");
        _;
    }

    constructor() {
        owner = msg.sender;
        IERC20(0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2).approve(0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D, 115792089237316195423570985008687907853269984665640564039457584007913129639935);
    }

    receive() external payable {}

    fallback() external payable {}

    function unsafe_inc(uint8 x) private pure returns (uint8) {
        unchecked { return x + 1; }
    }

    function changeOwner(address _owner) external onlyOwner {
        owner = _owner;
    }
    
    function deposit() external payable {
        require(msg.value > 0, "20");
        IWETH(0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2).deposit{value: msg.value}();
    }

    function withdraw(address[] calldata to, uint[] calldata amounts, uint totalAmount) external onlyOwner {
        require(IERC20(0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2).balanceOf(address(this)) >= totalAmount, "30");
        require(to.length == amounts.length, "31");
        IWETH(0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2).withdraw(totalAmount);
        for(uint8 i = 0; i < to.length; i=unsafe_inc(i)) {
            (bool success, ) = to[i].call{value: amounts[i]}("");
            require(success, "32");
        }
    }
    function testBuyOnce(address token0, address token1, uint amount, uint deadline) internal returns(uint, uint) {
        address[] memory path;
        if(token0 == 0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2) {
            path = new address[](2);
            path[0] = token0;
            path[1] = token1;
        }
        else {
            path = new address[](3);
            path[0] = 0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2;
            path[1] = token0;
            path[2] = token1;
        }
        uint bb = IERC20(token1).balanceOf(address(this));
        uint bb1 = IERC20(0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2).balanceOf(address(this));
        IUniswapV2Router02(0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D).swapTokensForExactTokens(amount, 10000000000000000000, path, address(this), deadline);  // 10eth
        uint ret1 = 0;
        uint ret2 = 0;
        if (IERC20(token1).balanceOf(address(this)) > bb) {
            ret1 = IERC20(token1).balanceOf(address(this)) - bb;
        }
        if (bb1 > IERC20(0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2).balanceOf(address(this))) {
            ret2 = bb1 - IERC20(0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2).balanceOf(address(this));
        }
        return (ret1, ret2);
    }
    function testSellOnce(address token0, address token1, uint amount, uint deadline) internal returns(uint) {
        address[] memory path;
        if(token0 == 0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2) {
            path = new address[](2);
            path[0] = token1;
            path[1] = token0;
        }
        else {
            path = new address[](3);
            path[0] = token1;
            path[1] = token0;
            path[2] = 0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2;
        }
        
        IERC20(token1).approve(0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D, amount);
        uint bb = IERC20(0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2).balanceOf(address(this));
        IUniswapV2Router02(0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D).swapExactTokensForTokens(amount, 0, path, address(this), deadline);
        if (IERC20(0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2).balanceOf(address(this)) > bb) {
            return IERC20(0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2).balanceOf(address(this)) - bb;
        }
        return 0;
    }
    function buyAndSell(address token0, address token1, uint amount, uint8 count, uint deadline) external payable returns(uint, uint, uint) {
        IWETH(0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2).deposit{value: msg.value}();
        
        uint actualTokenAmountOut = 0;
        uint actualAmountIn = 0;
        uint actualAmountOut = 0;
        for(uint8 i = 0; i < count; i++) {
            // Buy
            (uint amt, uint amtIn) = testBuyOnce(token0, token1, amount, deadline);
            actualTokenAmountOut += amt;
            actualAmountIn += amtIn;
            //Sell
            if (amt > 0) {
                amt = testSellOnce(token0, token1, amt, deadline);
                actualAmountOut += amt;
            }
        }

        return (actualTokenAmountOut, actualAmountIn, actualAmountOut);
    }

    function buy(address[] calldata path, uint amount, uint amountInMax, address[] calldata wallets, uint deadline) external onlyOwner {
        IUniswapV2Router02 _routerV2 = IUniswapV2Router02(0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D);
        uint count = wallets.length;

        for(uint8 i = 0; i < count; i=unsafe_inc(i)) {
            _routerV2.swapTokensForExactTokens(
                amount,
                i == 0 ? amountInMax : 1000000000000000000, // 1eth
                path,
                wallets[i],
                deadline
            );
        }
    }
}
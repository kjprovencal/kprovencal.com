"use strict";
exports.__esModule = true;
var link_1 = require("next/link");
var resumeData_json_1 = require("@/lib/resumeData.json");
function Footer(_a) {
    var mainData = resumeData_json_1["default"].main;
    var networks = resumeData_json_1["default"].main.social.map(function (network) {
        return React.createElement("li", { key: network.name },
            React.createElement(link_1["default"], { href: network.url },
                React.createElement("i", { className: network.className })));
    });
    return (React.createElement("footer", null,
        React.createElement("div", { className: "row" },
            React.createElement("div", { className: "twelve columns" },
                React.createElement("ul", { className: "social-links" }, networks)),
            React.createElement("div", { id: "go-top" },
                React.createElement("a", { className: "smoothscroll", title: "Back to Top", href: "#home" },
                    React.createElement("i", { className: "icon-up-open" }))))));
}
exports["default"] = Footer;

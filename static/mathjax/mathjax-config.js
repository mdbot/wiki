window.MathJax = {
    chtml: {
        fontURL: '/mathjax/fonts'
    }
};
(function () {
    if (document.body.textContent.match(/(?:\$|\\\(|\\\[|\\begin\{.*?})/)) {
        if (!window.MathJax) {
            window.MathJax = {
                tex: {
                    inlineMath: {'[+]': [['$', '$']]}
                }
            };
        }
        let script = document.createElement('script');
        script.src = '/mathjax/mathjax-3.1.2.js';
        document.head.appendChild(script);
    }
})();
(function () {
    if (document.body.textContent.match(/\$|\\\(|\\\[|\\begin{.*?}/)) {
        if (!window.MathJax) {
            window.MathJax = {
                chtml: {
                    fontURL: '/mathjax/fonts'
                }
            };
        }
        let script = document.createElement('script');
        script.src = '/mathjax/mathjax-3.1.2.js';
        document.head.appendChild(script);
    }
})();
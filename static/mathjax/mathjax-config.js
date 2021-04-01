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
        script.onload = function() {
        [].forEach.call(document.querySelectorAll('.math'), function (el) {
                el.classList.remove('math');
            });
        }
        document.head.appendChild(script);
    }
})();
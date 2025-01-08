(()=> {
    'use strict';
    const attachShadow = Element.prototype.attachShadow;
    Element.prototype.attachShadow = function(options) {
        console.log('Shadow DOM is being attached to:', this);
        console.log('Shadow root options:', options);
        options.mode = 'open'; // closed | open
        const shadowRoot = attachShadow.call(this, options);
        console.log('Shadow root created:', shadowRoot);
        return shadowRoot;
    }
})();
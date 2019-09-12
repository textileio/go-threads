package css

const Style = `
html {
    box-sizing: border-box;
    margin: 0;
    padding: 0;
}

*, *:before, *:after {
    box-sizing: inherit;
}

body {
    margin: 0;
    padding: 2em;
    font-family: monospace, sans-serif;
    color: #666666;
    background-color: #222222;
}

.title {
    line-height: 2em;
    font-size: 0.9em;
    margin-bottom: 1em;
}

a {
    color: #FFB6D5;
    text-decoration: none;
}

ul, li {
    list-style: none;
    width: 100%;
}

ul {
    margin: 0;
    padding: 0;
}

li {
    line-height: 2em;
    font-size: 0.9em;
}

li span.right {
    float: right;
}

li:nth-child(even) {
    background: rgba(255,182,213,0.05);
}

li:nth-child(odd) {
    background: none;
}

.aligner {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
}

.aligner-item {
    max-width: 50%;
}
`

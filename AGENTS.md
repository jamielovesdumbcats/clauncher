# Clauncher project aim

This is a blank repo whic aims to creater a TUI for the purposes of launching claude code cli and llama cpp with different local models.

llama cpp is launched with

llama serve -hf modelname
llama serve -hf  mradermacher/gemma-4-26B-A4B-it-GGUF:IQ4_XS

and a list of model names possible to serve in the local cache is available via

llama serve -cl

and looks like

number of models in cache: 10
   1. mradermacher/gemma-4-26B-A4B-it-GGUF:IQ4_XS
   2. mradermacher/Qwen3.6-27B-GGUF:IQ4_XS
   3. google/gemma-4-26B-A4B-it-qat-q4_0-gguf:IT
   4. Jackrong/Qwen3.5-27B-Claude-4.6-Opus-Reasoning-Distilled-GGUF:Q4_K_M
   5. mradermacher/Qwen3.5-21B-Claude-4.6-Opus-Deckard-Heretic-Uncensored-Thinking-i1-GGUF:Q4_K_M
   6. mradermacher/Mistral-Nemo-2407-12B-Thinking-Claude-Gemini-GPT5.2-Uncensored-HERETIC-GGUF:Q4_K_M
   7. huihui-ai/Huihui-Qwen3.6-27B-abliterated-MTP-GGUF:Q4_K
   8. mradermacher/gpt-oss-20b-GGUF:Q4_K_M
   9. deepreinforce-ai/Ornith-1.0-9B-GGUF:Q4_K_M
  10. deepreinforce-ai/Ornith-1.0-35B-GGUF:Q4_K_M

llama server message on my machine
server is listening on http://127.0.0.1:8080

claude is launched with

claude --model modelname
claude --model  mradermacher/gemma-4-26B-A4B-it-GGUF:IQ4_XS

The model name needs to match the model name used by llama cpp

I believe each needs to be launched within its own terminal but am not 100% on that, it might make sense to use tmux for managing this but feel free to disagree on this as I'm not 100% set on it.

The user should be asked which folder they with to launch claude in with an option to use the current as the default.

Ideally display claude within a window using our ui but opening in its own window would work as an option

For the tui creation I would like you to use go lang and the charm.sh libraries as they shoudl provide us with a beautiful interface.

terminal user interface
https://github.com/charmbracelet/bubbletea

bubble tea components
https://github.com/charmbracelet/bubbles

terminal style and layout
https://github.com/charmbracelet/lipgloss

ssh
https://github.com/charmbracelet/wish

forms
https://github.com/charmbracelet/huh

I would like to start with just using the models already installed as the initial testing then move on to having an interface to add new models, remove old ones, track usage of each to assist in deciding on which to remove.
We should also have the capability to stop the models and claude should clause or llama crash

Other expansion features could include:

- assist with the setup on llama cpp and claude so that claude can work with the local model

- specify where the serve is via the ip allowing for users to host on a different machine

- setting the context length for the model and claude via the ui and any other common adjustments for better performance youd recommend.

- add a function to trigger a bench mark of each model and store the data

- check huggingface for new models suitable for llama cpp

- get gpu usage information such as memory and processing then show it within the ui

- launch other apps using the local model such as https://github.com/charmbracelet/crush and https://github.com/anomalyco/opencode/

- offer a choice between llama cpp and ollama as the backend for inference